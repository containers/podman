package imagebuildah

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/containers/buildah"
	"github.com/containers/buildah/copier"
	"github.com/containers/buildah/define"
	buildahdocker "github.com/containers/buildah/docker"
	"github.com/containers/buildah/internal"
	"github.com/containers/buildah/internal/tmpdir"
	internalUtil "github.com/containers/buildah/internal/util"
	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/buildah/pkg/rusage"
	"github.com/containers/buildah/util"
	config "github.com/containers/common/pkg/config"
	cp "github.com/containers/image/v5/copy"
	imagedocker "github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/manifest"
	is "github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/chrootarchive"
	"github.com/containers/storage/pkg/unshare"
	docker "github.com/fsouza/go-dockerclient"
	buildkitparser "github.com/moby/buildkit/frontend/dockerfile/parser"
	digest "github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/openshift/imagebuilder"
	"github.com/openshift/imagebuilder/dockerfile/command"
	"github.com/openshift/imagebuilder/dockerfile/parser"
	"github.com/sirupsen/logrus"
)

// StageExecutor bundles up what we need to know when executing one stage of a
// (possibly multi-stage) build.
// Each stage may need to produce an image to be used as the base in a later
// stage (with the last stage's image being the end product of the build), and
// it may need to leave its working container in place so that the container's
// root filesystem's contents can be used as the source for a COPY instruction
// in a later stage.
// Each stage has its own base image, so it starts with its own configuration
// and set of volumes.
// If we're naming the result of the build, only the last stage will apply that
// name to the image that it produces.
type StageExecutor struct {
	ctx                   context.Context
	executor              *Executor
	log                   func(format string, args ...interface{})
	index                 int
	stages                imagebuilder.Stages
	name                  string
	builder               *buildah.Builder
	preserved             int
	volumes               imagebuilder.VolumeSet
	volumeCache           map[string]string
	volumeCacheInfo       map[string]os.FileInfo
	mountPoint            string
	output                string
	containerIDs          []string
	stage                 *imagebuilder.Stage
	didExecute            bool
	argsFromContainerfile []string
}

// Preserve informs the stage executor that from this point on, it needs to
// ensure that only COPY and ADD instructions can modify the contents of this
// directory or anything below it.
// The StageExecutor handles this by caching the contents of directories which
// have been marked this way before executing a RUN instruction, invalidating
// that cache when an ADD or COPY instruction sets any location under the
// directory as the destination, and using the cache to reset the contents of
// the directory tree after processing each RUN instruction.
// It would be simpler if we could just mark the directory as a read-only bind
// mount of itself during Run(), but the directory is expected to be remain
// writeable while the RUN instruction is being handled, even if any changes
// made within the directory are ultimately discarded.
func (s *StageExecutor) Preserve(path string) error {
	logrus.Debugf("PRESERVE %q", path)
	if s.volumes.Covers(path) {
		// This path is already a subdirectory of a volume path that
		// we're already preserving, so there's nothing new to be done
		// except ensure that it exists.
		createdDirPerms := os.FileMode(0755)
		if err := copier.Mkdir(s.mountPoint, filepath.Join(s.mountPoint, path), copier.MkdirOptions{ChmodNew: &createdDirPerms}); err != nil {
			return fmt.Errorf("ensuring volume path exists: %w", err)
		}
		if err := s.volumeCacheInvalidate(path); err != nil {
			return fmt.Errorf("ensuring volume path %q is preserved: %w", filepath.Join(s.mountPoint, path), err)
		}
		return nil
	}
	// Figure out where the cache for this volume would be stored.
	s.preserved++
	cacheDir, err := s.executor.store.ContainerDirectory(s.builder.ContainerID)
	if err != nil {
		return fmt.Errorf("unable to locate temporary directory for container")
	}
	cacheFile := filepath.Join(cacheDir, fmt.Sprintf("volume%d.tar", s.preserved))
	// Save info about the top level of the location that we'll be archiving.
	var archivedPath string

	// Try and resolve the symlink (if one exists)
	// Set archivedPath and path based on whether a symlink is found or not
	if evaluated, err := copier.Eval(s.mountPoint, filepath.Join(s.mountPoint, path), copier.EvalOptions{}); err == nil {
		symLink, err := filepath.Rel(s.mountPoint, evaluated)
		if err != nil {
			return fmt.Errorf("making evaluated path %q relative to %q: %w", evaluated, s.mountPoint, err)
		}
		if strings.HasPrefix(symLink, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("evaluated path %q was not below %q", evaluated, s.mountPoint)
		}
		archivedPath = evaluated
		path = string(os.PathSeparator) + symLink
	} else {
		return fmt.Errorf("evaluating path %q: %w", path, err)
	}

	st, err := os.Stat(archivedPath)
	if errors.Is(err, os.ErrNotExist) {
		createdDirPerms := os.FileMode(0755)
		if err = copier.Mkdir(s.mountPoint, archivedPath, copier.MkdirOptions{ChmodNew: &createdDirPerms}); err != nil {
			return fmt.Errorf("ensuring volume path exists: %w", err)
		}
		st, err = os.Stat(archivedPath)
	}
	if err != nil {
		logrus.Debugf("error reading info about %q: %v", archivedPath, err)
		return err
	}
	s.volumeCacheInfo[path] = st
	if !s.volumes.Add(path) {
		// This path is not a subdirectory of a volume path that we're
		// already preserving, so adding it to the list should work.
		return fmt.Errorf("adding %q to the volume cache", path)
	}
	s.volumeCache[path] = cacheFile
	// Now prune cache files for volumes that are now supplanted by this one.
	removed := []string{}
	for cachedPath := range s.volumeCache {
		// Walk our list of cached volumes, and check that they're
		// still in the list of locations that we need to cache.
		found := false
		for _, volume := range s.volumes {
			if volume == cachedPath {
				// We need to keep this volume's cache.
				found = true
				break
			}
		}
		if !found {
			// We don't need to keep this volume's cache.  Make a
			// note to remove it.
			removed = append(removed, cachedPath)
		}
	}
	// Actually remove the caches that we decided to remove.
	for _, cachedPath := range removed {
		archivedPath := filepath.Join(s.mountPoint, cachedPath)
		logrus.Debugf("no longer need cache of %q in %q", archivedPath, s.volumeCache[cachedPath])
		if err := os.Remove(s.volumeCache[cachedPath]); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		delete(s.volumeCache, cachedPath)
	}
	return nil
}

// Remove any volume cache item which will need to be re-saved because we're
// writing to part of it.
func (s *StageExecutor) volumeCacheInvalidate(path string) error {
	invalidated := []string{}
	for cachedPath := range s.volumeCache {
		if strings.HasPrefix(path, cachedPath+string(os.PathSeparator)) {
			invalidated = append(invalidated, cachedPath)
		}
	}
	for _, cachedPath := range invalidated {
		if err := os.Remove(s.volumeCache[cachedPath]); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		archivedPath := filepath.Join(s.mountPoint, cachedPath)
		logrus.Debugf("invalidated volume cache %q for %q from %q", archivedPath, path, s.volumeCache[cachedPath])
	}
	return nil
}

// Save the contents of each of the executor's list of volumes for which we
// don't already have a cache file.
func (s *StageExecutor) volumeCacheSaveVFS() (mounts []specs.Mount, err error) {
	for cachedPath, cacheFile := range s.volumeCache {
		archivedPath, err := copier.Eval(s.mountPoint, filepath.Join(s.mountPoint, cachedPath), copier.EvalOptions{})
		if err != nil {
			return nil, fmt.Errorf("evaluating volume path: %w", err)
		}
		relativePath, err := filepath.Rel(s.mountPoint, archivedPath)
		if err != nil {
			return nil, fmt.Errorf("converting %q into a path relative to %q: %w", archivedPath, s.mountPoint, err)
		}
		if strings.HasPrefix(relativePath, ".."+string(os.PathSeparator)) {
			return nil, fmt.Errorf("converting %q into a path relative to %q", archivedPath, s.mountPoint)
		}
		_, err = os.Stat(cacheFile)
		if err == nil {
			logrus.Debugf("contents of volume %q are already cached in %q", archivedPath, cacheFile)
			continue
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		createdDirPerms := os.FileMode(0755)
		if err := copier.Mkdir(s.mountPoint, archivedPath, copier.MkdirOptions{ChmodNew: &createdDirPerms}); err != nil {
			return nil, fmt.Errorf("ensuring volume path exists: %w", err)
		}
		logrus.Debugf("caching contents of volume %q in %q", archivedPath, cacheFile)
		cache, err := os.Create(cacheFile)
		if err != nil {
			return nil, err
		}
		defer cache.Close()
		rc, err := chrootarchive.Tar(archivedPath, nil, s.mountPoint)
		if err != nil {
			return nil, fmt.Errorf("archiving %q: %w", archivedPath, err)
		}
		defer rc.Close()
		_, err = io.Copy(cache, rc)
		if err != nil {
			return nil, fmt.Errorf("archiving %q to %q: %w", archivedPath, cacheFile, err)
		}
		mount := specs.Mount{
			Source:      archivedPath,
			Destination: string(os.PathSeparator) + relativePath,
			Type:        "bind",
			Options:     []string{"private"},
		}
		mounts = append(mounts, mount)
	}
	return nil, nil
}

// Restore the contents of each of the executor's list of volumes.
func (s *StageExecutor) volumeCacheRestoreVFS() (err error) {
	for cachedPath, cacheFile := range s.volumeCache {
		archivedPath, err := copier.Eval(s.mountPoint, filepath.Join(s.mountPoint, cachedPath), copier.EvalOptions{})
		if err != nil {
			return fmt.Errorf("evaluating volume path: %w", err)
		}
		logrus.Debugf("restoring contents of volume %q from %q", archivedPath, cacheFile)
		cache, err := os.Open(cacheFile)
		if err != nil {
			return err
		}
		defer cache.Close()
		if err := copier.Remove(s.mountPoint, archivedPath, copier.RemoveOptions{All: true}); err != nil {
			return err
		}
		createdDirPerms := os.FileMode(0755)
		if err := copier.Mkdir(s.mountPoint, archivedPath, copier.MkdirOptions{ChmodNew: &createdDirPerms}); err != nil {
			return err
		}
		err = chrootarchive.Untar(cache, archivedPath, nil)
		if err != nil {
			return fmt.Errorf("extracting archive at %q: %w", archivedPath, err)
		}
		if st, ok := s.volumeCacheInfo[cachedPath]; ok {
			if err := os.Chmod(archivedPath, st.Mode()); err != nil {
				return err
			}
			uid := 0
			gid := 0
			if st.Sys() != nil {
				uid = util.UID(st)
				gid = util.GID(st)
			}
			if err := os.Chown(archivedPath, uid, gid); err != nil {
				return err
			}
			if err := os.Chtimes(archivedPath, st.ModTime(), st.ModTime()); err != nil {
				return err
			}
		}
	}
	return nil
}

// Save the contents of each of the executor's list of volumes for which we
// don't already have a cache file.
func (s *StageExecutor) volumeCacheSaveOverlay() (mounts []specs.Mount, err error) {
	for cachedPath := range s.volumeCache {
		err = copier.Mkdir(s.mountPoint, filepath.Join(s.mountPoint, cachedPath), copier.MkdirOptions{})
		if err != nil {
			return nil, fmt.Errorf("ensuring volume exists: %w", err)
		}
		volumePath := filepath.Join(s.mountPoint, cachedPath)
		mount := specs.Mount{
			Source:      volumePath,
			Destination: cachedPath,
			Options:     []string{"O", "private"},
		}
		mounts = append(mounts, mount)
	}
	return mounts, nil
}

// Reset the contents of each of the executor's list of volumes.
func (s *StageExecutor) volumeCacheRestoreOverlay() error {
	return nil
}

// Save the contents of each of the executor's list of volumes for which we
// don't already have a cache file.
func (s *StageExecutor) volumeCacheSave() (mounts []specs.Mount, err error) {
	switch s.executor.store.GraphDriverName() {
	case "overlay":
		return s.volumeCacheSaveOverlay()
	}
	return s.volumeCacheSaveVFS()
}

// Reset the contents of each of the executor's list of volumes.
func (s *StageExecutor) volumeCacheRestore() error {
	switch s.executor.store.GraphDriverName() {
	case "overlay":
		return s.volumeCacheRestoreOverlay()
	}
	return s.volumeCacheRestoreVFS()
}

// Copy copies data into the working tree.  The "Download" field is how
// imagebuilder tells us the instruction was "ADD" and not "COPY".
func (s *StageExecutor) Copy(excludes []string, copies ...imagebuilder.Copy) error {
	s.builder.ContentDigester.Restart()
	return s.performCopy(excludes, copies...)
}

func (s *StageExecutor) performCopy(excludes []string, copies ...imagebuilder.Copy) error {
	copiesExtend := []imagebuilder.Copy{}
	for _, copy := range copies {
		if err := s.volumeCacheInvalidate(copy.Dest); err != nil {
			return err
		}
		var sources []string
		// The From field says to read the content from another
		// container.  Update the ID mappings and
		// all-content-comes-from-below-this-directory value.
		var idMappingOptions *define.IDMappingOptions
		var copyExcludes []string
		stripSetuid := false
		stripSetgid := false
		preserveOwnership := false
		contextDir := s.executor.contextDir
		// If we are copying files via heredoc syntax, then
		// its time to create these temporary files on host
		// and copy these to container
		if len(copy.Files) > 0 {
			// If we are copying files from heredoc syntax, there
			// maybe regular files from context as well so split and
			// process them differently
			if len(copy.Src) > len(copy.Files) {
				regularSources := []string{}
				for _, src := range copy.Src {
					// If this source is not a heredoc, then it is a regular file from
					// build context or from another stage (`--from=`) so treat this differently.
					if !strings.HasPrefix(src, "<<") {
						regularSources = append(regularSources, src)
					}
				}
				copyEntry := copy
				// Remove heredoc if any, since we are already processing them
				// so create new entry with sources containing regular files
				// only, since regular files can have different context then
				// heredoc files.
				copyEntry.Files = nil
				copyEntry.Src = regularSources
				copiesExtend = append(copiesExtend, copyEntry)
			}
			copySources := []string{}
			for _, file := range copy.Files {
				data := file.Data
				// remove first break line added while parsing heredoc
				data = strings.TrimPrefix(data, "\n")
				// add breakline when heredoc ends for docker compat
				data = data + "\n"
				tmpFile, err := os.Create(filepath.Join(parse.GetTempDir(), path.Base(filepath.ToSlash(file.Name))))
				if err != nil {
					return fmt.Errorf("unable to create tmp file for COPY instruction at %q: %w", parse.GetTempDir(), err)
				}
				err = tmpFile.Chmod(0644) // 644 is consistent with buildkit
				if err != nil {
					tmpFile.Close()
					return fmt.Errorf("unable to chmod tmp file created for COPY instruction at %q: %w", tmpFile.Name(), err)
				}
				defer os.Remove(tmpFile.Name())
				_, err = tmpFile.WriteString(data)
				if err != nil {
					tmpFile.Close()
					return fmt.Errorf("unable to write contents of heredoc file at %q: %w", tmpFile.Name(), err)
				}
				copySources = append(copySources, filepath.Base(tmpFile.Name()))
				tmpFile.Close()
			}
			contextDir = parse.GetTempDir()
			copy.Src = copySources
		}

		if len(copy.From) > 0 && len(copy.Files) == 0 {
			// If from has an argument within it, resolve it to its
			// value.  Otherwise just return the value found.
			from, fromErr := imagebuilder.ProcessWord(copy.From, s.stage.Builder.Arguments())
			if fromErr != nil {
				return fmt.Errorf("unable to resolve argument %q: %w", copy.From, fromErr)
			}
			var additionalBuildContext *define.AdditionalBuildContext
			if foundContext, ok := s.executor.additionalBuildContexts[from]; ok {
				additionalBuildContext = foundContext
			} else {
				// Maybe index is given in COPY --from=index
				// if that's the case check if provided index
				// exists and if stage short_name matches any
				// additionalContext replace stage with additional
				// build context.
				if index, err := strconv.Atoi(from); err == nil {
					from = s.stages[index].Name
				}
				if foundContext, ok := s.executor.additionalBuildContexts[from]; ok {
					additionalBuildContext = foundContext
				}
			}
			if additionalBuildContext != nil {
				if !additionalBuildContext.IsImage {
					contextDir = additionalBuildContext.Value
					if additionalBuildContext.IsURL {
						// Check if following buildContext was already
						// downloaded before in any other RUN step. If not
						// download it and populate DownloadCache field for
						// future RUN steps.
						if additionalBuildContext.DownloadedCache == "" {
							// additional context contains a tar file
							// so download and explode tar to buildah
							// temp and point context to that.
							path, subdir, err := define.TempDirForURL(tmpdir.GetTempDir(), internal.BuildahExternalArtifactsDir, additionalBuildContext.Value)
							if err != nil {
								return fmt.Errorf("unable to download context from external source %q: %w", additionalBuildContext.Value, err)
							}
							// point context dir to the extracted path
							contextDir = filepath.Join(path, subdir)
							// populate cache for next RUN step
							additionalBuildContext.DownloadedCache = contextDir
						} else {
							contextDir = additionalBuildContext.DownloadedCache
						}
					} else {
						// This points to a path on the filesystem
						// Check to see if there's a .containerignore
						// file, update excludes for this stage before
						// proceeding
						buildContextExcludes, _, err := parse.ContainerIgnoreFile(additionalBuildContext.Value, "", nil)
						if err != nil {
							return err
						}
						excludes = append(excludes, buildContextExcludes...)
					}
				} else {
					copy.From = additionalBuildContext.Value
				}
			}
			if additionalBuildContext == nil {
				if isStage, err := s.executor.waitForStage(s.ctx, from, s.stages[:s.index]); isStage && err != nil {
					return err
				}
				if other, ok := s.executor.stages[from]; ok && other.index < s.index {
					contextDir = other.mountPoint
					idMappingOptions = &other.builder.IDMappingOptions
				} else if builder, ok := s.executor.containerMap[copy.From]; ok {
					contextDir = builder.MountPoint
					idMappingOptions = &builder.IDMappingOptions
				} else {
					return fmt.Errorf("the stage %q has not been built", copy.From)
				}
			} else if additionalBuildContext.IsImage {
				// Image was selected as additionalContext so only process image.
				mountPoint, err := s.getImageRootfs(s.ctx, copy.From)
				if err != nil {
					return err
				}
				contextDir = mountPoint
			}
			// Original behaviour of buildah still stays true for COPY irrespective of additional context.
			preserveOwnership = true
			copyExcludes = excludes
		} else {
			copyExcludes = append(s.executor.excludes, excludes...)
			stripSetuid = true // did this change between 18.06 and 19.03?
			stripSetgid = true // did this change between 18.06 and 19.03?
		}
		if copy.Download {
			logrus.Debugf("ADD %#v, %#v", excludes, copy)
		} else {
			logrus.Debugf("COPY %#v, %#v", excludes, copy)
		}
		for _, src := range copy.Src {
			if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
				// Source is a URL, allowed for ADD but not COPY.
				if copy.Download {
					sources = append(sources, src)
				} else {
					// returns an error to be compatible with docker
					return fmt.Errorf("source can't be a URL for COPY")
				}
			} else {
				sources = append(sources, filepath.Join(contextDir, src))
			}
		}
		options := buildah.AddAndCopyOptions{
			Chmod:             copy.Chmod,
			Chown:             copy.Chown,
			Checksum:          copy.Checksum,
			PreserveOwnership: preserveOwnership,
			ContextDir:        contextDir,
			Excludes:          copyExcludes,
			IgnoreFile:        s.executor.ignoreFile,
			IDMappingOptions:  idMappingOptions,
			StripSetuidBit:    stripSetuid,
			StripSetgidBit:    stripSetgid,
		}
		if err := s.builder.Add(copy.Dest, copy.Download, options, sources...); err != nil {
			return err
		}
	}
	if len(copiesExtend) > 0 {
		// If we found heredocs and regularfiles together
		// in same statement then we produced new copies to
		// process regular files separately since they need
		// different context.
		return s.performCopy(excludes, copiesExtend...)
	}
	return nil
}

// Returns a map of StageName/ImageName:internal.StageMountDetails for RunOpts if any --mount with from is provided
// Stage can automatically cleanup this mounts when a stage is removed
// check if RUN contains `--mount` with `from`. If yes pre-mount images or stages from executor for Run.
// stages mounted here will we used be Run().
func (s *StageExecutor) runStageMountPoints(mountList []string) (map[string]internal.StageMountDetails, error) {
	stageMountPoints := make(map[string]internal.StageMountDetails)
	for _, flag := range mountList {
		if strings.Contains(flag, "from") {
			arr := strings.SplitN(flag, ",", 2)
			if len(arr) < 2 {
				return nil, fmt.Errorf("Invalid --mount command: %s", flag)
			}
			tokens := strings.Split(flag, ",")
			for _, val := range tokens {
				kv := strings.SplitN(val, "=", 2)
				switch kv[0] {
				case "from":
					if len(kv) == 1 {
						return nil, fmt.Errorf("unable to resolve argument for `from=`: bad argument")
					}
					if kv[1] == "" {
						return nil, fmt.Errorf("unable to resolve argument for `from=`: from points to an empty value")
					}
					from, fromErr := imagebuilder.ProcessWord(kv[1], s.stage.Builder.Arguments())
					if fromErr != nil {
						return nil, fmt.Errorf("unable to resolve argument %q: %w", kv[1], fromErr)
					}
					// If additional buildContext contains this
					// give priority to that and break if additional
					// is not an external image.
					if additionalBuildContext, ok := s.executor.additionalBuildContexts[from]; ok {
						if additionalBuildContext.IsImage {
							mountPoint, err := s.getImageRootfs(s.ctx, additionalBuildContext.Value)
							if err != nil {
								return nil, fmt.Errorf("%s from=%s: image found with that name", flag, from)
							}
							// The `from` in stageMountPoints should point
							// to `mountPoint` replaced from additional
							// build-context. Reason: Parser will use this
							//  `from` to refer from stageMountPoints map later.
							stageMountPoints[from] = internal.StageMountDetails{IsStage: false, DidExecute: true, MountPoint: mountPoint}
							break
						} else {
							// Most likely this points to path on filesystem
							// or external tar archive, Treat it as a stage
							// nothing is different for this. So process and
							// point mountPoint to path on host and it will
							// be automatically handled correctly by since
							// GetBindMount will honor IsStage:false while
							// processing stageMountPoints.
							mountPoint := additionalBuildContext.Value
							if additionalBuildContext.IsURL {
								// Check if following buildContext was already
								// downloaded before in any other RUN step. If not
								// download it and populate DownloadCache field for
								// future RUN steps.
								if additionalBuildContext.DownloadedCache == "" {
									// additional context contains a tar file
									// so download and explode tar to buildah
									// temp and point context to that.
									path, subdir, err := define.TempDirForURL(tmpdir.GetTempDir(), internal.BuildahExternalArtifactsDir, additionalBuildContext.Value)
									if err != nil {
										return nil, fmt.Errorf("unable to download context from external source %q: %w", additionalBuildContext.Value, err)
									}
									// point context dir to the extracted path
									mountPoint = filepath.Join(path, subdir)
									// populate cache for next RUN step
									additionalBuildContext.DownloadedCache = mountPoint
								} else {
									mountPoint = additionalBuildContext.DownloadedCache
								}
							}
							stageMountPoints[from] = internal.StageMountDetails{IsStage: true, DidExecute: true, MountPoint: mountPoint}
							break
						}
					}
					// If the source's name corresponds to the
					// result of an earlier stage, wait for that
					// stage to finish being built.
					if isStage, err := s.executor.waitForStage(s.ctx, from, s.stages[:s.index]); isStage && err != nil {
						return nil, err
					}
					if otherStage, ok := s.executor.stages[from]; ok && otherStage.index < s.index {
						stageMountPoints[from] = internal.StageMountDetails{IsStage: true, DidExecute: otherStage.didExecute, MountPoint: otherStage.mountPoint}
						break
					} else {
						mountPoint, err := s.getImageRootfs(s.ctx, from)
						if err != nil {
							return nil, fmt.Errorf("%s from=%s: no stage or image found with that name", flag, from)
						}
						stageMountPoints[from] = internal.StageMountDetails{IsStage: false, DidExecute: true, MountPoint: mountPoint}
						break
					}
				default:
					continue
				}
			}
		}
	}
	return stageMountPoints, nil
}

func (s *StageExecutor) createNeededHeredocMountsForRun(files []imagebuilder.File) ([]Mount, error) {
	mountResult := []Mount{}
	for _, file := range files {
		f, err := os.CreateTemp(parse.GetTempDir(), "buildahheredoc")
		if err != nil {
			return nil, err
		}
		if _, err := f.WriteString(file.Data); err != nil {
			f.Close()
			return nil, err
		}
		err = f.Chmod(0755)
		if err != nil {
			f.Close()
			return nil, err
		}
		// dest path is same as buildkit for compat
		dest := filepath.Join("/dev/pipes/", filepath.Base(f.Name()))
		mount := Mount{Destination: dest, Type: define.TypeBind, Source: f.Name(), Options: append(define.BindOptions, "rprivate", "z", "Z")}
		mountResult = append(mountResult, mount)
		f.Close()
	}
	return mountResult, nil
}

// Run executes a RUN instruction using the stage's current working container
// as a root directory.
func (s *StageExecutor) Run(run imagebuilder.Run, config docker.Config) error {
	logrus.Debugf("RUN %#v, %#v", run, config)
	args := run.Args
	heredocMounts := []Mount{}
	if len(run.Files) > 0 {
		if heredoc := buildkitparser.MustParseHeredoc(args[0]); heredoc != nil {
			if strings.HasPrefix(run.Files[0].Data, "#!") || strings.HasPrefix(run.Files[0].Data, "\n#!") {
				// This is a single heredoc with a shebang, so create a file
				// and run it.
				heredocMount, err := s.createNeededHeredocMountsForRun(run.Files)
				if err != nil {
					return err
				}
				args = []string{heredocMount[0].Destination}
				heredocMounts = append(heredocMounts, heredocMount...)
			} else {
				args = []string{run.Files[0].Data}
			}
		} else {
			full := args[0]
			for _, file := range run.Files {
				full += file.Data + "\n" + file.Name
			}
			args = []string{full}
		}
	}
	stageMountPoints, err := s.runStageMountPoints(run.Mounts)
	if err != nil {
		return err
	}
	if s.builder == nil {
		return fmt.Errorf("no build container available")
	}
	stdin := s.executor.in
	if stdin == nil {
		devNull, err := os.Open(os.DevNull)
		if err != nil {
			return fmt.Errorf("opening %q for reading: %v", os.DevNull, err)
		}
		defer devNull.Close()
		stdin = devNull
	}
	namespaceOptions := append([]define.NamespaceOption{}, s.executor.namespaceOptions...)
	options := buildah.RunOptions{
		Args:             s.executor.runtimeArgs,
		Cmd:              config.Cmd,
		ContextDir:       s.executor.contextDir,
		ConfigureNetwork: s.executor.configureNetwork,
		Entrypoint:       config.Entrypoint,
		Env:              config.Env,
		Hostname:         config.Hostname,
		Logger:           s.executor.logger,
		Mounts:           append([]Mount{}, s.executor.transientMounts...),
		NamespaceOptions: namespaceOptions,
		NoHostname:       s.executor.noHostname,
		NoHosts:          s.executor.noHosts,
		NoPivot:          os.Getenv("BUILDAH_NOPIVOT") != "",
		Quiet:            s.executor.quiet,
		RunMounts:        run.Mounts,
		Runtime:          s.executor.runtime,
		Secrets:          s.executor.secrets,
		SSHSources:       s.executor.sshsources,
		StageMountPoints: stageMountPoints,
		Stderr:           s.executor.err,
		Stdin:            stdin,
		Stdout:           s.executor.out,
		SystemContext:    s.executor.systemContext,
		Terminal:         buildah.WithoutTerminal,
		User:             config.User,
		WorkingDir:       config.WorkingDir,
	}

	// Honor `RUN --network=<>`.
	switch run.Network {
	case "host":
		options.NamespaceOptions.AddOrReplace(define.NamespaceOption{Name: "network", Host: true})
		options.ConfigureNetwork = define.NetworkEnabled
	case "none":
		options.ConfigureNetwork = define.NetworkDisabled
	case "", "default":
		// do nothing
	default:
		return fmt.Errorf(`unsupported value %q for "RUN --network", must be either "host" or "none"`, run.Network)
	}

	if config.NetworkDisabled {
		options.ConfigureNetwork = buildah.NetworkDisabled
	}

	if run.Shell {
		if len(config.Shell) > 0 && s.builder.Format == define.Dockerv2ImageManifest {
			args = append(config.Shell, args...)
		} else {
			args = append([]string{"/bin/sh", "-c"}, args...)
		}
	}
	mounts, err := s.volumeCacheSave()
	if err != nil {
		return err
	}
	options.Mounts = append(options.Mounts, mounts...)
	if len(heredocMounts) > 0 {
		options.Mounts = append(options.Mounts, heredocMounts...)
	}
	err = s.builder.Run(args, options)
	if err2 := s.volumeCacheRestore(); err2 != nil {
		if err == nil {
			return err2
		}
	}
	return err
}

// UnrecognizedInstruction is called when we encounter an instruction that the
// imagebuilder parser didn't understand.
func (s *StageExecutor) UnrecognizedInstruction(step *imagebuilder.Step) error {
	errStr := fmt.Sprintf("Build error: Unknown instruction: %q ", strings.ToUpper(step.Command))
	err := fmt.Sprintf(errStr+"%#v", step)
	if s.executor.ignoreUnrecognizedInstructions {
		logrus.Debugf(err)
		return nil
	}

	switch logrus.GetLevel() {
	case logrus.ErrorLevel:
		s.executor.logger.Errorf(errStr)
	case logrus.DebugLevel:
		logrus.Debugf(err)
	default:
		s.executor.logger.Errorf("+(UNHANDLED LOGLEVEL) %#v", step)
	}

	return fmt.Errorf(err)
}

// prepare creates a working container based on the specified image, or if one
// isn't specified, the first argument passed to the first FROM instruction we
// can find in the stage's parsed tree.
func (s *StageExecutor) prepare(ctx context.Context, from string, initializeIBConfig, rebase, preserveBaseImageAnnotations bool, pullPolicy define.PullPolicy) (builder *buildah.Builder, err error) {
	stage := s.stage
	ib := stage.Builder
	node := stage.Node

	if from == "" {
		base, err := ib.From(node)
		if err != nil {
			logrus.Debugf("prepare(node.Children=%#v)", node.Children)
			return nil, fmt.Errorf("determining starting point for build: %w", err)
		}
		from = base
	}
	displayFrom := from

	// stage.Name will be a numeric string for all stages without an "AS" clause
	asImageName := stage.Name
	if asImageName != "" {
		if _, err := strconv.Atoi(asImageName); err != nil {
			displayFrom = from + " AS " + asImageName
		}
	}

	if initializeIBConfig && rebase {
		logrus.Debugf("FROM %#v", displayFrom)
		if !s.executor.quiet {
			s.log("FROM %s", displayFrom)
		}
	}

	builderSystemContext := s.executor.systemContext
	// get platform string from stage
	if stage.Builder.Platform != "" {
		os, arch, variant, err := parse.Platform(stage.Builder.Platform)
		if err != nil {
			return nil, fmt.Errorf("unable to parse platform %q: %w", stage.Builder.Platform, err)
		}
		if arch != "" || variant != "" {
			builderSystemContext.ArchitectureChoice = arch
			builderSystemContext.VariantChoice = variant
		}
		if os != "" {
			builderSystemContext.OSChoice = os
		}
	}

	builderOptions := buildah.BuilderOptions{
		Args:                  ib.Args,
		FromImage:             from,
		GroupAdd:              s.executor.groupAdd,
		PullPolicy:            pullPolicy,
		ContainerSuffix:       s.executor.containerSuffix,
		Registry:              s.executor.registry,
		BlobDirectory:         s.executor.blobDirectory,
		SignaturePolicyPath:   s.executor.signaturePolicyPath,
		ReportWriter:          s.executor.reportWriter,
		SystemContext:         builderSystemContext,
		Isolation:             s.executor.isolation,
		NamespaceOptions:      s.executor.namespaceOptions,
		ConfigureNetwork:      s.executor.configureNetwork,
		CNIPluginPath:         s.executor.cniPluginPath,
		CNIConfigDir:          s.executor.cniConfigDir,
		NetworkInterface:      s.executor.networkInterface,
		IDMappingOptions:      s.executor.idmappingOptions,
		CommonBuildOpts:       s.executor.commonBuildOptions,
		DefaultMountsFilePath: s.executor.defaultMountsFilePath,
		Format:                s.executor.outputFormat,
		Capabilities:          s.executor.capabilities,
		Devices:               s.executor.devices,
		MaxPullRetries:        s.executor.maxPullPushRetries,
		PullRetryDelay:        s.executor.retryPullPushDelay,
		OciDecryptConfig:      s.executor.ociDecryptConfig,
		Logger:                s.executor.logger,
		ProcessLabel:          s.executor.processLabel,
		MountLabel:            s.executor.mountLabel,
		PreserveBaseImageAnns: preserveBaseImageAnnotations,
	}

	builder, err = buildah.NewBuilder(ctx, s.executor.store, builderOptions)
	if err != nil {
		return nil, fmt.Errorf("creating build container: %w", err)
	}

	// If executor's ProcessLabel and MountLabel is empty means this is the first stage
	// Make sure we share first stage's ProcessLabel and MountLabel with all other subsequent stages
	// Doing this will ensure and one stage in same build can mount another stage even if `selinux`
	// is enabled.

	if s.executor.mountLabel == "" && s.executor.processLabel == "" {
		s.executor.mountLabel = builder.MountLabel
		s.executor.processLabel = builder.ProcessLabel
	}

	if initializeIBConfig {
		volumes := map[string]struct{}{}
		for _, v := range builder.Volumes() {
			volumes[v] = struct{}{}
		}
		ports := map[docker.Port]struct{}{}
		for _, p := range builder.Ports() {
			ports[docker.Port(p)] = struct{}{}
		}
		dConfig := docker.Config{
			Hostname:     builder.Hostname(),
			Domainname:   builder.Domainname(),
			User:         builder.User(),
			Env:          builder.Env(),
			Cmd:          builder.Cmd(),
			Image:        from,
			Volumes:      volumes,
			WorkingDir:   builder.WorkDir(),
			Entrypoint:   builder.Entrypoint(),
			Healthcheck:  (*docker.HealthConfig)(builder.Healthcheck()),
			Labels:       builder.Labels(),
			Shell:        builder.Shell(),
			StopSignal:   builder.StopSignal(),
			OnBuild:      builder.OnBuild(),
			ExposedPorts: ports,
		}
		var rootfs *docker.RootFS
		if builder.Docker.RootFS != nil {
			rootfs = &docker.RootFS{
				Type: builder.Docker.RootFS.Type,
			}
			for _, id := range builder.Docker.RootFS.DiffIDs {
				rootfs.Layers = append(rootfs.Layers, id.String())
			}
		}
		dImage := docker.Image{
			Parent:          builder.FromImageID,
			ContainerConfig: dConfig,
			Container:       builder.Container,
			Author:          builder.Maintainer(),
			Architecture:    builder.Architecture(),
			RootFS:          rootfs,
		}
		dImage.Config = &dImage.ContainerConfig
		err = ib.FromImage(&dImage, node)
		if err != nil {
			if err2 := builder.Delete(); err2 != nil {
				logrus.Debugf("error deleting container which we failed to update: %v", err2)
			}
			return nil, fmt.Errorf("updating build context: %w", err)
		}
	}
	mountPoint, err := builder.Mount(builder.MountLabel)
	if err != nil {
		if err2 := builder.Delete(); err2 != nil {
			logrus.Debugf("error deleting container which we failed to mount: %v", err2)
		}
		return nil, fmt.Errorf("mounting new container: %w", err)
	}
	if rebase {
		// Make this our "current" working container.
		s.mountPoint = mountPoint
		s.builder = builder
	}
	logrus.Debugln("Container ID:", builder.ContainerID)
	return builder, nil
}

// Delete deletes the stage's working container, if we have one.
func (s *StageExecutor) Delete() (err error) {
	if s.builder != nil {
		err = s.builder.Delete()
		s.builder = nil
	}
	return err
}

// stepRequiresLayer indicates whether or not the step should be followed by
// committing a layer container when creating an intermediate image.
func (*StageExecutor) stepRequiresLayer(step *imagebuilder.Step) bool {
	switch strings.ToUpper(step.Command) {
	case "ADD", "COPY", "RUN":
		return true
	}
	return false
}

// getImageRootfs checks for an image matching the passed-in name in local
// storage.  If it isn't found, it pulls down a copy.  Then, if we don't have a
// working container root filesystem based on the image, it creates one.  Then
// it returns that root filesystem's location.
func (s *StageExecutor) getImageRootfs(ctx context.Context, image string) (mountPoint string, err error) {
	if builder, ok := s.executor.containerMap[image]; ok {
		return builder.MountPoint, nil
	}
	builder, err := s.prepare(ctx, image, false, false, false, s.executor.pullPolicy)
	if err != nil {
		return "", err
	}
	s.executor.containerMap[image] = builder
	return builder.MountPoint, nil
}

// getContentSummary generates content summary for cases where we added content and need
// to get summary with updated digests.
func (s *StageExecutor) getContentSummaryAfterAddingContent() string {
	contentType, digest := s.builder.ContentDigester.Digest()
	summary := contentType
	if digest != "" {
		if summary != "" {
			summary = summary + ":"
		}
		summary = summary + digest.Encoded()
		logrus.Debugf("added content %s", summary)
	}
	return summary
}

// Execute runs each of the steps in the stage's parsed tree, in turn.
func (s *StageExecutor) Execute(ctx context.Context, base string) (imgID string, ref reference.Canonical, onlyBaseImg bool, err error) {
	var resourceUsage rusage.Rusage
	stage := s.stage
	ib := stage.Builder
	checkForLayers := s.executor.layers && s.executor.useCache
	moreStages := s.index < len(s.stages)-1
	lastStage := !moreStages
	onlyBaseImage := false
	imageIsUsedLater := moreStages && (s.executor.baseMap[stage.Name] || s.executor.baseMap[strconv.Itoa(stage.Position)])
	rootfsIsUsedLater := moreStages && (s.executor.rootfsMap[stage.Name] || s.executor.rootfsMap[strconv.Itoa(stage.Position)])

	// If the base image's name corresponds to the result of an earlier
	// stage, make sure that stage has finished building an image, and
	// substitute that image's ID for the base image's name here and force
	// the pull policy to "never" to avoid triggering an error when it's
	// set to "always", which doesn't make sense for image IDs.
	// If not, then go on assuming that it's just a regular image that's
	// either in local storage, or one that we have to pull from a
	// registry, subject to the passed-in pull policy.
	if isStage, err := s.executor.waitForStage(ctx, base, s.stages[:s.index]); isStage && err != nil {
		return "", nil, false, err
	}
	pullPolicy := s.executor.pullPolicy
	s.executor.stagesLock.Lock()
	var preserveBaseImageAnnotationsAtStageStart bool
	if stageImage, isPreviousStage := s.executor.imageMap[base]; isPreviousStage {
		base = stageImage
		pullPolicy = define.PullNever
		preserveBaseImageAnnotationsAtStageStart = true
	}
	s.executor.stagesLock.Unlock()

	// Set things up so that we can log resource usage as we go.
	logRusage := func() {
		if rusage.Supported() {
			usage, err := rusage.Get()
			if err != nil {
				fmt.Fprintf(s.executor.out, "error gathering resource usage information: %v\n", err)
				return
			}
			if s.executor.rusageLogFile != nil {
				fmt.Fprintf(s.executor.rusageLogFile, "%s\n", rusage.FormatDiff(usage.Subtract(resourceUsage)))
			}
			resourceUsage = usage
		}
	}

	// Start counting resource usage before we potentially pull a base image.
	if rusage.Supported() {
		if resourceUsage, err = rusage.Get(); err != nil {
			return "", nil, false, err
		}
		// Log the final incremental resource usage counter before we return.
		defer logRusage()
	}

	// Create the (first) working container for this stage.  Reinitializing
	// the imagebuilder configuration may alter the list of steps we have,
	// so take a snapshot of them *after* that.
	if _, err := s.prepare(ctx, base, true, true, preserveBaseImageAnnotationsAtStageStart, pullPolicy); err != nil {
		return "", nil, false, err
	}
	children := stage.Node.Children

	// A helper function to only log "COMMIT" as an explicit step if it's
	// the very last step of a (possibly multi-stage) build.
	logCommit := func(output string, instruction int) {
		moreInstructions := instruction < len(children)-1
		if moreInstructions || moreStages {
			return
		}
		commitMessage := "COMMIT"
		if output != "" {
			commitMessage = fmt.Sprintf("%s %s", commitMessage, output)
		}
		logrus.Debugf(commitMessage)
		if !s.executor.quiet {
			s.log(commitMessage)
		}
	}
	// logCachePulled produces build log for cases when `--cache-from`
	// is used and a valid intermediate image is pulled from remote source.
	logCachePulled := func(cacheKey string, remote reference.Named) {
		if !s.executor.quiet {
			cachePullMessage := "--> Cache pulled from remote"
			fmt.Fprintf(s.executor.out, "%s %s\n", cachePullMessage, fmt.Sprintf("%s:%s", remote.String(), cacheKey))
		}
	}
	// logCachePush produces build log for cases when `--cache-to`
	// is used and a valid intermediate image is pushed tp remote source.
	logCachePush := func(cacheKey string) {
		if !s.executor.quiet {
			cachePushMessage := "--> Pushing cache"
			fmt.Fprintf(s.executor.out, "%s %s\n", cachePushMessage, fmt.Sprintf("%s:%s", s.executor.cacheTo, cacheKey))
		}
	}
	logCacheHit := func(cacheID string) {
		if !s.executor.quiet {
			cacheHitMessage := "--> Using cache"
			fmt.Fprintf(s.executor.out, "%s %s\n", cacheHitMessage, cacheID)
		}
	}
	logImageID := func(imgID string) {
		if len(imgID) > 12 {
			imgID = imgID[:12]
		}
		if s.executor.iidfile == "" {
			fmt.Fprintf(s.executor.out, "--> %s\n", imgID)
		}
	}

	// Parse and populate buildOutputOption if needed
	var buildOutputOption define.BuildOutputOption
	canGenerateBuildOutput := (s.executor.buildOutput != "" && lastStage)
	if canGenerateBuildOutput {
		logrus.Debugf("Generating custom build output with options %q", s.executor.buildOutput)
		buildOutputOption, err = parse.GetBuildOutput(s.executor.buildOutput)
		if err != nil {
			return "", nil, false, fmt.Errorf("failed to parse build output: %w", err)
		}
	}

	if len(children) == 0 {
		// There are no steps.
		if s.builder.FromImageID == "" || s.executor.squash || s.executor.confidentialWorkload.Convert || len(s.executor.labels) > 0 || len(s.executor.annotations) > 0 || len(s.executor.unsetEnvs) > 0 || len(s.executor.unsetLabels) > 0 {
			// We either don't have a base image, or we need to
			// transform the contents of the base image, or we need
			// to make some changes to just the config blob.  Whichever
			// is the case, we need to commit() to create a new image.
			logCommit(s.output, -1)
			// No base image means there's nothing to put in a
			// layer, so don't create one.
			emptyLayer := (s.builder.FromImageID == "")
			if imgID, ref, err = s.commit(ctx, s.getCreatedBy(nil, ""), emptyLayer, s.output, s.executor.squash, lastStage); err != nil {
				return "", nil, false, fmt.Errorf("committing base container: %w", err)
			}
		} else {
			// We don't need to squash or otherwise transform the
			// base image, and the image wouldn't be modified by
			// the command line options, so just reuse the base
			// image.
			logCommit(s.output, -1)
			if imgID, ref, err = s.tagExistingImage(ctx, s.builder.FromImageID, s.output); err != nil {
				return "", nil, onlyBaseImage, err
			}
			onlyBaseImage = true
		}
		// Generate build output from the new image, or the preexisting
		// one if we didn't actually do anything, if needed.
		if canGenerateBuildOutput {
			if err := s.generateBuildOutput(buildOutputOption); err != nil {
				return "", nil, onlyBaseImage, err
			}
		}
		logImageID(imgID)
	}

	for i, node := range children {
		logRusage()
		moreInstructions := i < len(children)-1
		lastInstruction := !moreInstructions
		// Resolve any arguments in this instruction.
		step := ib.Step()
		if err := step.Resolve(node); err != nil {
			return "", nil, false, fmt.Errorf("resolving step %+v: %w", *node, err)
		}
		logrus.Debugf("Parsed Step: %+v", *step)
		if !s.executor.quiet {
			s.log("%s", step.Original)
		}

		// Check if there's a --from if the step command is COPY.
		// Also check the chmod and the chown flags for validity.
		for _, flag := range step.Flags {
			command := strings.ToUpper(step.Command)
			// chmod, chown and from flags should have an '=' sign, '--chmod=', '--chown=' or '--from='
			if command == "COPY" && (flag == "--chmod" || flag == "--chown" || flag == "--from") {
				return "", nil, false, fmt.Errorf("COPY only supports the --chmod=<permissions> --chown=<uid:gid> and the --from=<image|stage> flags")
			}
			if command == "ADD" && (flag == "--chmod" || flag == "--chown" || flag == "--checksum") {
				return "", nil, false, fmt.Errorf("ADD only supports the --chmod=<permissions>, --chown=<uid:gid>, and --checksum=<checksum> flags")
			}
			if strings.Contains(flag, "--from") && command == "COPY" {
				arr := strings.Split(flag, "=")
				if len(arr) != 2 {
					return "", nil, false, fmt.Errorf("%s: invalid --from flag %q, should be --from=<name|stage>", command, flag)
				}
				// If arr[1] has an argument within it, resolve it to its
				// value.  Otherwise just return the value found.
				from, fromErr := imagebuilder.ProcessWord(arr[1], s.stage.Builder.Arguments())
				if fromErr != nil {
					return "", nil, false, fmt.Errorf("unable to resolve argument %q: %w", arr[1], fromErr)
				}

				// Before looking into additional context
				// also account if the index is given instead
				// of name so convert index in --from=<index>
				// to name.
				if index, err := strconv.Atoi(from); err == nil {
					from = s.stages[index].Name
				}
				// If additional buildContext contains this
				// give priority to that and break if additional
				// is not an external image.
				if additionalBuildContext, ok := s.executor.additionalBuildContexts[from]; ok {
					if !additionalBuildContext.IsImage {
						// We don't need to pull this
						// since this additional context
						// is not an image.
						break
					} else {
						// replace with image set in build context
						from = additionalBuildContext.Value
						if _, err := s.getImageRootfs(ctx, from); err != nil {
							return "", nil, false, fmt.Errorf("%s --from=%s: no stage or image found with that name", command, from)
						}
						break
					}
				}

				// If the source's name corresponds to the
				// result of an earlier stage, wait for that
				// stage to finish being built.
				if isStage, err := s.executor.waitForStage(ctx, from, s.stages[:s.index]); isStage && err != nil {
					return "", nil, false, err
				}
				if otherStage, ok := s.executor.stages[from]; ok && otherStage.index < s.index {
					break
				} else if _, err = s.getImageRootfs(ctx, from); err != nil {
					return "", nil, false, fmt.Errorf("%s --from=%s: no stage or image found with that name", command, from)
				}
				break
			}
		}

		// Determine if there are any RUN instructions to be run after
		// this step.  If not, we won't have to bother preserving the
		// contents of any volumes declared between now and when we
		// finish.
		noRunsRemaining := false
		if moreInstructions {
			noRunsRemaining = !ib.RequiresStart(&parser.Node{Children: children[i+1:]})
		}

		// If we're doing a single-layer build, just process the
		// instruction.
		if !s.executor.layers {
			s.didExecute = true
			err := ib.Run(step, s, noRunsRemaining)
			if err != nil {
				logrus.Debugf("Error building at step %+v: %v", *step, err)
				return "", nil, false, fmt.Errorf("building at STEP \"%s\": %w", step.Message, err)
			}
			// In case we added content, retrieve its digest.
			addedContentSummary := s.getContentSummaryAfterAddingContent()
			if moreInstructions {
				// There are still more instructions to process
				// for this stage.  Make a note of the
				// instruction in the history that we'll write
				// for the image when we eventually commit it.
				timestamp := time.Now().UTC()
				if s.executor.timestamp != nil {
					timestamp = *s.executor.timestamp
				}
				s.builder.AddPrependedEmptyLayer(&timestamp, s.getCreatedBy(node, addedContentSummary), "", "")
				continue
			} else {
				// This is the last instruction for this stage,
				// so we should commit this container to create
				// an image, but only if it's the last stage,
				// or if it's used as the basis for a later
				// stage.
				if lastStage || imageIsUsedLater {
					logCommit(s.output, i)
					imgID, ref, err = s.commit(ctx, s.getCreatedBy(node, addedContentSummary), false, s.output, s.executor.squash, lastStage && lastInstruction)
					if err != nil {
						return "", nil, false, fmt.Errorf("committing container for step %+v: %w", *step, err)
					}
					logImageID(imgID)
					// Generate build output if needed.
					if canGenerateBuildOutput {
						if err := s.generateBuildOutput(buildOutputOption); err != nil {
							return "", nil, false, err
						}
					}
				} else {
					imgID = ""
				}
				break
			}
		}

		// We're in a multi-layered build.
		s.didExecute = false
		var (
			commitName                string
			cacheID                   string
			cacheKey                  string
			pulledAndUsedCacheImage   bool
			err                       error
			rebase                    bool
			addedContentSummary       string
			canMatchCacheOnlyAfterRun bool
		)

		// Only attempt to find cache if its needed, this part is needed
		// so that if a step is using RUN --mount and mounts content from
		// previous stages then it uses the freshly built stage instead
		// of re-using the older stage from the store.
		avoidLookingCache := false
		var mounts []string
		for _, a := range node.Flags {
			arg, err := imagebuilder.ProcessWord(a, s.stage.Builder.Arguments())
			if err != nil {
				return "", nil, false, err
			}
			switch {
			case strings.HasPrefix(arg, "--mount="):
				mount := strings.TrimPrefix(arg, "--mount=")
				mounts = append(mounts, mount)
			default:
				continue
			}
		}
		stageMountPoints, err := s.runStageMountPoints(mounts)
		if err != nil {
			return "", nil, false, err
		}
		for _, mountPoint := range stageMountPoints {
			if mountPoint.DidExecute {
				avoidLookingCache = true
			}
		}

		needsCacheKey := (len(s.executor.cacheFrom) != 0 || len(s.executor.cacheTo) != 0) && !avoidLookingCache

		// If we have to commit for this instruction, only assign the
		// stage's configured output name to the last layer.
		if lastInstruction {
			commitName = s.output
		}

		// If --cache-from or --cache-to is specified make sure to populate
		// cacheKey since it will be used either while pulling or pushing the
		// cache images.
		if needsCacheKey {
			cacheKey, err = s.generateCacheKey(ctx, node, addedContentSummary, s.stepRequiresLayer(step))
			if err != nil {
				return "", nil, false, fmt.Errorf("failed while generating cache key: %w", err)
			}
		}
		// Check if there's already an image based on our parent that
		// has the same change that we're about to make, so far as we
		// can tell.
		// Only do this if the step we are on is not an ARG step,
		// we need to call ib.Run() to correctly put the args together before
		// determining if a cached layer with the same build args already exists
		// and that is done in the if block below.
		if checkForLayers && step.Command != "arg" && !(s.executor.squash && lastInstruction && lastStage) && !avoidLookingCache {
			// For `COPY` and `ADD`, history entries include digests computed from
			// the content that's copied in.  We need to compute that information so that
			// it can be used to evaluate the cache, which means we need to go ahead
			// and copy the content.
			canMatchCacheOnlyAfterRun = (step.Command == command.Add || step.Command == command.Copy)
			if canMatchCacheOnlyAfterRun {
				s.didExecute = true
				if err = ib.Run(step, s, noRunsRemaining); err != nil {
					logrus.Debugf("Error building at step %+v: %v", *step, err)
					return "", nil, false, fmt.Errorf("building at STEP \"%s\": %w", step.Message, err)
				}
				// Retrieve the digest info for the content that we just copied
				// into the rootfs.
				addedContentSummary = s.getContentSummaryAfterAddingContent()
				// regenerate cache key with updated content summary
				if needsCacheKey {
					cacheKey, err = s.generateCacheKey(ctx, node, addedContentSummary, s.stepRequiresLayer(step))
					if err != nil {
						return "", nil, false, fmt.Errorf("failed while generating cache key: %w", err)
					}
				}
			}
			cacheID, err = s.intermediateImageExists(ctx, node, addedContentSummary, s.stepRequiresLayer(step))
			if err != nil {
				return "", nil, false, fmt.Errorf("checking if cached image exists from a previous build: %w", err)
			}
			// All the best effort to find cache on localstorage have failed try pulling
			// cache from remote repo if `--cache-from` was configured.
			if cacheID == "" && len(s.executor.cacheFrom) != 0 {
				// only attempt to use cache again if pulling was successful
				// otherwise do nothing and attempt to run the step, err != nil
				// is ignored and will be automatically logged for --log-level debug
				if ref, id, err := s.pullCache(ctx, cacheKey); ref != nil && id != "" && err == nil {
					logCachePulled(cacheKey, ref)
					cacheID, err = s.intermediateImageExists(ctx, node, addedContentSummary, s.stepRequiresLayer(step))
					if err != nil {
						return "", nil, false, fmt.Errorf("checking if cached image exists from a previous build: %w", err)
					}
					if cacheID != "" {
						pulledAndUsedCacheImage = true
					}
				}
			}
		}

		// If we didn't find a cache entry, or we need to add content
		// to find the digest of the content to check for a cached
		// image, run the step so that we can check if the result
		// matches a cache.
		// We already called ib.Run() for the `canMatchCacheOnlyAfterRun`
		// cases above, so we shouldn't do it again.
		if cacheID == "" && !canMatchCacheOnlyAfterRun {
			// Process the instruction directly.
			s.didExecute = true
			if err = ib.Run(step, s, noRunsRemaining); err != nil {
				logrus.Debugf("Error building at step %+v: %v", *step, err)
				return "", nil, false, fmt.Errorf("building at STEP \"%s\": %w", step.Message, err)
			}

			// In case we added content, retrieve its digest.
			addedContentSummary = s.getContentSummaryAfterAddingContent()
			// regenerate cache key with updated content summary
			if needsCacheKey {
				cacheKey, err = s.generateCacheKey(ctx, node, addedContentSummary, s.stepRequiresLayer(step))
				if err != nil {
					return "", nil, false, fmt.Errorf("failed while generating cache key: %w", err)
				}
			}

			// Check if there's already an image based on our parent that
			// has the same change that we just made.
			if checkForLayers && !avoidLookingCache {
				cacheID, err = s.intermediateImageExists(ctx, node, addedContentSummary, s.stepRequiresLayer(step))
				if err != nil {
					return "", nil, false, fmt.Errorf("checking if cached image exists from a previous build: %w", err)
				}
				// All the best effort to find cache on localstorage have failed try pulling
				// cache from remote repo if `--cache-from` was configured and cacheKey was
				// generated again after adding content summary.
				if cacheID == "" && len(s.executor.cacheFrom) != 0 {
					// only attempt to use cache again if pulling was successful
					// otherwise do nothing and attempt to run the step, err != nil
					// is ignored and will be automatically logged for --log-level debug
					if ref, id, err := s.pullCache(ctx, cacheKey); ref != nil && id != "" && err == nil {
						logCachePulled(cacheKey, ref)
						cacheID, err = s.intermediateImageExists(ctx, node, addedContentSummary, s.stepRequiresLayer(step))
						if err != nil {
							return "", nil, false, fmt.Errorf("checking if cached image exists from a previous build: %w", err)
						}
						if cacheID != "" {
							pulledAndUsedCacheImage = true
						}
					}
				}
			}
		} else {
			// This log line is majorly here so we can verify in tests
			// that our cache is performing in the most optimal way for
			// various cases.
			logrus.Debugf("Found a cache hit in the first iteration with id %s", cacheID)
			// If the instruction would affect our configuration,
			// process the configuration change so that, if we fall
			// off the cache path, the filesystem changes from the
			// last cache image will be all that we need, since we
			// still don't want to restart using the image's
			// configuration blob.
			if !s.stepRequiresLayer(step) {
				s.didExecute = true
				err := ib.Run(step, s, noRunsRemaining)
				if err != nil {
					logrus.Debugf("Error building at step %+v: %v", *step, err)
					return "", nil, false, fmt.Errorf("building at STEP \"%s\": %w", step.Message, err)
				}
			}
		}

		// Note: If the build has squash, we must try to re-use as many layers as possible if cache is found.
		// So only perform commit if it's the lastInstruction of lastStage.
		if cacheID != "" {
			logCacheHit(cacheID)
			// A suitable cached image was found, so we can just
			// reuse it.  If we need to add a name to the resulting
			// image because it's the last step in this stage, add
			// the name to the image.
			imgID = cacheID
			if commitName != "" {
				logCommit(commitName, i)
				if imgID, ref, err = s.tagExistingImage(ctx, cacheID, commitName); err != nil {
					return "", nil, false, err
				}
			}
		} else {
			// We're not going to find any more cache hits, so we
			// can stop looking for them.
			checkForLayers = false
			// Create a new image, maybe with a new layer, with the
			// name for this stage if it's the last instruction.
			logCommit(s.output, i)
			// While committing we always set squash to false here
			// because at this point we want to save history for
			// layers even if its a squashed build so that they
			// can be part of the build cache.
			imgID, ref, err = s.commit(ctx, s.getCreatedBy(node, addedContentSummary), !s.stepRequiresLayer(step), commitName, false, lastStage && lastInstruction)
			if err != nil {
				return "", nil, false, fmt.Errorf("committing container for step %+v: %w", *step, err)
			}
			// Generate build output if needed.
			if canGenerateBuildOutput {
				if err := s.generateBuildOutput(buildOutputOption); err != nil {
					return "", nil, false, err
				}
			}
		}

		// Following step is just built and was not used from
		// cache so check if --cache-to was specified if yes
		// then attempt pushing this cache to remote repo and
		// fail accordingly.
		//
		// Or
		//
		// Try to push this cache to remote repository only
		// if cache was present on local storage and not
		// pulled from remote source while processing this
		if len(s.executor.cacheTo) != 0 && (!pulledAndUsedCacheImage || cacheID == "") && needsCacheKey {
			logCachePush(cacheKey)
			if err = s.pushCache(ctx, imgID, cacheKey); err != nil {
				return "", nil, false, err
			}
		}

		if lastInstruction && lastStage {
			if s.executor.squash || s.executor.confidentialWorkload.Convert {
				// Create a squashed version of this image
				// if we're supposed to create one and this
				// is the last instruction of the last stage.
				imgID, ref, err = s.commit(ctx, s.getCreatedBy(node, addedContentSummary), !s.stepRequiresLayer(step), commitName, true, lastStage && lastInstruction)
				if err != nil {
					return "", nil, false, fmt.Errorf("committing final squash step %+v: %w", *step, err)
				}
				// Generate build output if needed.
				if canGenerateBuildOutput {
					if err := s.generateBuildOutput(buildOutputOption); err != nil {
						return "", nil, false, err
					}
				}
			} else if cacheID != "" {
				// If we found a valid cache hit and this is lastStage
				// and not a squashed build then there is no opportunity
				// for us to perform a `commit` later in the code since
				// everything will be used from cache.
				//
				// If above statement is true and --output was provided
				// then generate output manually since there is no opportunity
				// for us to perform `commit` anywhere in the code.
				// Generate build output if needed.
				if canGenerateBuildOutput {
					if err := s.generateBuildOutput(buildOutputOption); err != nil {
						return "", nil, false, err
					}
				}
			}
		}

		logImageID(imgID)

		// Update our working container to be based off of the cached
		// image, if we might need to use it as a basis for the next
		// instruction, or if we need the root filesystem to match the
		// image contents for the sake of a later stage that wants to
		// copy content from it.
		rebase = moreInstructions || rootfsIsUsedLater

		if rebase {
			// Since we either committed the working container or
			// are about to replace it with one based on a cached
			// image, add the current working container's ID to the
			// list of successful intermediate containers that
			// we'll clean up later.
			s.containerIDs = append(s.containerIDs, s.builder.ContainerID)

			// Prepare for the next step or subsequent phases by
			// creating a new working container with the
			// just-committed or updated cached image as its new
			// base image.
			// Enforce pull "never" since we already have an image
			// ID that we really should not be pulling anymore (see
			// containers/podman/issues/10307).
			if _, err := s.prepare(ctx, imgID, false, true, true, define.PullNever); err != nil {
				return "", nil, false, fmt.Errorf("preparing container for next step: %w", err)
			}
		}
	}

	return imgID, ref, onlyBaseImage, nil
}

func historyEntriesEqual(base, derived v1.History) bool {
	if base.CreatedBy != derived.CreatedBy {
		return false
	}
	if base.Comment != derived.Comment {
		return false
	}
	if base.Author != derived.Author {
		return false
	}
	if base.EmptyLayer != derived.EmptyLayer {
		return false
	}
	if base.Created != nil && derived.Created == nil {
		return false
	}
	if base.Created == nil && derived.Created != nil {
		return false
	}
	if base.Created != nil && derived.Created != nil && !base.Created.Equal(*derived.Created) {
		return false
	}
	return true
}

// historyAndDiffIDsMatch returns true if a candidate history matches the
// history of our base image (if we have one), plus the current instruction,
// and if the list of diff IDs for the images do for the part of the history
// that we're comparing.
// Used to verify whether a cache of the intermediate image exists and whether
// to run the build again.
func (s *StageExecutor) historyAndDiffIDsMatch(baseHistory []v1.History, baseDiffIDs []digest.Digest, child *parser.Node, history []v1.History, diffIDs []digest.Digest, addedContentSummary string, buildAddsLayer bool) bool {
	// our history should be as long as the base's, plus one entry for what
	// we're doing
	if len(history) != len(baseHistory)+1 {
		return false
	}
	// check that each entry in the base history corresponds to an entry in
	// our history, and count how many of them add a layer diff
	expectedDiffIDs := 0
	for i := range baseHistory {
		if !historyEntriesEqual(baseHistory[i], history[i]) {
			return false
		}
		if !baseHistory[i].EmptyLayer {
			expectedDiffIDs++
		}
	}
	if len(baseDiffIDs) != expectedDiffIDs {
		return false
	}
	if buildAddsLayer {
		// we're adding a layer, so we should have exactly one more
		// layer than the base image
		if len(diffIDs) != expectedDiffIDs+1 {
			return false
		}
	} else {
		// we're not adding a layer, so we should have exactly the same
		// layers as the base image
		if len(diffIDs) != expectedDiffIDs {
			return false
		}
	}
	// compare the diffs for the layers that we should have in common
	for i := range baseDiffIDs {
		if diffIDs[i] != baseDiffIDs[i] {
			return false
		}
	}
	return history[len(baseHistory)].CreatedBy == s.getCreatedBy(child, addedContentSummary)
}

// getCreatedBy returns the command the image at node will be created by.  If
// the passed-in CompositeDigester is not nil, it is assumed to have the digest
// information for the content if the node is ADD or COPY.
func (s *StageExecutor) getCreatedBy(node *parser.Node, addedContentSummary string) string {
	if node == nil {
		return "/bin/sh"
	}
	switch strings.ToUpper(node.Value) {
	case "ARG":
		for _, variable := range strings.Fields(node.Original) {
			if variable != "ARG" {
				s.argsFromContainerfile = append(s.argsFromContainerfile, variable)
			}
		}
		buildArgs := s.getBuildArgsKey()
		return "/bin/sh -c #(nop) ARG " + buildArgs
	case "RUN":
		buildArgs := s.getBuildArgsResolvedForRun()
		if buildArgs != "" {
			return "|" + strconv.Itoa(len(strings.Split(buildArgs, " "))) + " " + buildArgs + " /bin/sh -c " + node.Original[4:]
		}
		return "/bin/sh -c " + node.Original[4:]
	case "ADD", "COPY":
		destination := node
		for destination.Next != nil {
			destination = destination.Next
		}
		return "/bin/sh -c #(nop) " + strings.ToUpper(node.Value) + " " + addedContentSummary + " in " + destination.Value + " "
	default:
		return "/bin/sh -c #(nop) " + node.Original
	}
}

// getBuildArgs returns a string of the build-args specified during the build process
// it excludes any build-args that were not used in the build process
// values for args are overridden by the values specified using ENV.
// Reason: Values from ENV will always override values specified arg.
func (s *StageExecutor) getBuildArgsResolvedForRun() string {
	var envs []string
	configuredEnvs := make(map[string]string)
	dockerConfig := s.stage.Builder.Config()

	for _, env := range dockerConfig.Env {
		splitv := strings.SplitN(env, "=", 2)
		if len(splitv) == 2 {
			configuredEnvs[splitv[0]] = splitv[1]
		}
	}

	for key, value := range s.stage.Builder.Args {
		if _, ok := s.stage.Builder.AllowedArgs[key]; ok {
			// if value was in image it will be given higher priority
			// so please embed that into build history
			_, inImage := configuredEnvs[key]
			if inImage {
				envs = append(envs, fmt.Sprintf("%s=%s", key, configuredEnvs[key]))
			} else {
				// By default everything must be added to history.
				// Following variable is configured to false only for special cases.
				addToHistory := true

				// Following value is being assigned from build-args,
				// check if this key belongs to any of the predefined allowlist args e.g Proxy Variables
				// and if that arg is not manually set in Containerfile/Dockerfile
				// then don't write its value to history.
				// Following behaviour ensures parity with docker/buildkit.
				for _, variable := range config.ProxyEnv {
					if key == variable {
						// found in predefined args
						// so don't add to history
						// unless user did explicit `ARG <some-predefined-proxy-variable>`
						addToHistory = false
						for _, processedArg := range s.argsFromContainerfile {
							if key == processedArg {
								addToHistory = true
							}
						}
					}
				}
				if addToHistory {
					envs = append(envs, fmt.Sprintf("%s=%s", key, value))
				}
			}
		}
	}
	sort.Strings(envs)
	return strings.Join(envs, " ")
}

// getBuildArgs key returns the set of args which were specified during the
// build process, formatted for inclusion in the build history
func (s *StageExecutor) getBuildArgsKey() string {
	var args []string
	for key := range s.stage.Builder.Args {
		if _, ok := s.stage.Builder.AllowedArgs[key]; ok {
			args = append(args, key)
		}
	}
	sort.Strings(args)
	return strings.Join(args, " ")
}

// tagExistingImage adds names to an image already in the store
func (s *StageExecutor) tagExistingImage(ctx context.Context, cacheID, output string) (string, reference.Canonical, error) {
	// If we don't need to attach a name to the image, just return the cache ID.
	if output == "" {
		return cacheID, nil, nil
	}

	// Get the destination image reference.
	dest, err := s.executor.resolveNameToImageRef(output)
	if err != nil {
		return "", nil, err
	}

	policyContext, err := util.GetPolicyContext(s.executor.systemContext)
	if err != nil {
		return "", nil, err
	}
	defer func() {
		if destroyErr := policyContext.Destroy(); destroyErr != nil {
			if err == nil {
				err = destroyErr
			} else {
				err = fmt.Errorf("%v: %w", destroyErr.Error(), err)
			}
		}
	}()

	// Look up the source image, expecting it to be in local storage
	src, err := is.Transport.ParseStoreReference(s.executor.store, cacheID)
	if err != nil {
		return "", nil, fmt.Errorf("getting source imageReference for %q: %w", cacheID, err)
	}
	options := cp.Options{
		RemoveSignatures: true, // more like "ignore signatures", since they don't get removed when src and dest are the same image
	}
	manifestBytes, err := cp.Image(ctx, policyContext, dest, src, &options)
	if err != nil {
		return "", nil, fmt.Errorf("copying image %q: %w", cacheID, err)
	}
	manifestDigest, err := manifest.Digest(manifestBytes)
	if err != nil {
		return "", nil, fmt.Errorf("computing digest of manifest for image %q: %w", cacheID, err)
	}
	_, img, err := is.ResolveReference(dest)
	if err != nil {
		return "", nil, fmt.Errorf("locating new copy of image %q (i.e., %q): %w", cacheID, transports.ImageName(dest), err)
	}
	var ref reference.Canonical
	if dref := dest.DockerReference(); dref != nil {
		if ref, err = reference.WithDigest(dref, manifestDigest); err != nil {
			return "", nil, fmt.Errorf("computing canonical reference for new image %q (i.e., %q): %w", cacheID, transports.ImageName(dest), err)
		}
	}
	return img.ID, ref, nil
}

// generateCacheKey returns a computed digest for the current STEP
// running its history and diff against a hash algorithm and this
// generated CacheKey is further used by buildah to lock and decide
// tag for the intermediate image which can be pushed and pulled to/from
// the remote repository.
func (s *StageExecutor) generateCacheKey(ctx context.Context, currNode *parser.Node, addedContentDigest string, buildAddsLayer bool) (string, error) {
	hash := sha256.New()
	var baseHistory []v1.History
	var diffIDs []digest.Digest
	var manifestType string
	var err error
	if s.builder.FromImageID != "" {
		manifestType, baseHistory, diffIDs, err = s.executor.getImageTypeAndHistoryAndDiffIDs(ctx, s.builder.FromImageID)
		if err != nil {
			return "", fmt.Errorf("getting history of base image %q: %w", s.builder.FromImageID, err)
		}
		for i := 0; i < len(diffIDs); i++ {
			fmt.Fprintln(hash, diffIDs[i].String())
		}
	}
	createdBy := s.getCreatedBy(currNode, addedContentDigest)
	fmt.Fprintf(hash, "%t", buildAddsLayer)
	fmt.Fprintln(hash, createdBy)
	fmt.Fprintln(hash, manifestType)
	for _, element := range baseHistory {
		fmt.Fprintln(hash, element.CreatedBy)
		fmt.Fprintln(hash, element.Author)
		fmt.Fprintln(hash, element.Comment)
		fmt.Fprintln(hash, element.Created)
		fmt.Fprintf(hash, "%t", element.EmptyLayer)
		fmt.Fprintln(hash)
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// cacheImageReference is internal function which generates ImageReference from Named repo sources
// and a tag.
func cacheImageReferences(repos []reference.Named, cachekey string) ([]types.ImageReference, error) {
	var result []types.ImageReference
	for _, repo := range repos {
		tagged, err := reference.WithTag(repo, cachekey)
		if err != nil {
			return nil, fmt.Errorf("failed generating tagged reference for %q: %w", repo, err)
		}
		dest, err := imagedocker.NewReference(tagged)
		if err != nil {
			return nil, fmt.Errorf("failed generating docker reference for %q: %w", tagged, err)
		}
		result = append(result, dest)
	}
	return result, nil
}

// pushCache takes the image id of intermediate image and attempts
// to perform push at the remote repository with cacheKey as the tag.
// Returns error if fails otherwise returns nil.
func (s *StageExecutor) pushCache(ctx context.Context, src, cacheKey string) error {
	destList, err := cacheImageReferences(s.executor.cacheTo, cacheKey)
	if err != nil {
		return err
	}
	for _, dest := range destList {
		logrus.Debugf("trying to push cache to dest: %+v from src:%+v", dest, src)
		options := buildah.PushOptions{
			Compression:         s.executor.compression,
			SignaturePolicyPath: s.executor.signaturePolicyPath,
			Store:               s.executor.store,
			SystemContext:       s.executor.systemContext,
			BlobDirectory:       s.executor.blobDirectory,
			SignBy:              s.executor.signBy,
			MaxRetries:          s.executor.maxPullPushRetries,
			RetryDelay:          s.executor.retryPullPushDelay,
		}
		ref, digest, err := buildah.Push(ctx, src, dest, options)
		if err != nil {
			return fmt.Errorf("failed pushing cache to %q: %w", dest, err)
		}
		logrus.Debugf("successfully pushed cache to dest: %+v with ref:%+v and digest: %v", dest, ref, digest)
	}
	return nil
}

// pullCache takes the image source of the cache assuming tag
// already points to the valid cacheKey and pulls the image to
// local storage only if it was not already present on local storage
// or a newer version of cache was found in the upstream repo. If new
// image was pulled function returns image id otherwise returns empty
// string "" or error if any error was encontered while pulling the cache.
func (s *StageExecutor) pullCache(ctx context.Context, cacheKey string) (reference.Named, string, error) {
	srcList, err := cacheImageReferences(s.executor.cacheFrom, cacheKey)
	if err != nil {
		return nil, "", err
	}
	for _, src := range srcList {
		logrus.Debugf("trying to pull cache from remote repo: %+v", src.DockerReference())
		options := buildah.PullOptions{
			SignaturePolicyPath: s.executor.signaturePolicyPath,
			Store:               s.executor.store,
			SystemContext:       s.executor.systemContext,
			BlobDirectory:       s.executor.blobDirectory,
			MaxRetries:          s.executor.maxPullPushRetries,
			RetryDelay:          s.executor.retryPullPushDelay,
			AllTags:             false,
			ReportWriter:        nil,
			PullPolicy:          define.PullIfNewer,
		}
		id, err := buildah.Pull(ctx, src.DockerReference().String(), options)
		if err != nil {
			logrus.Debugf("failed pulling cache from source %s: %v", src, err)
			continue // failed pulling this one try next
			//return "", fmt.Errorf("failed while pulling cache from %q: %w", src, err)
		}
		logrus.Debugf("successfully pulled cache from repo %s: %s", src, id)
		return src.DockerReference(), id, nil
	}
	return nil, "", fmt.Errorf("failed pulling cache from all available sources %q", srcList)
}

// intermediateImageExists returns true if an intermediate image of currNode exists in the image store from a previous build.
// It verifies this by checking the parent of the top layer of the image and the history.
func (s *StageExecutor) intermediateImageExists(ctx context.Context, currNode *parser.Node, addedContentDigest string, buildAddsLayer bool) (string, error) {
	// Get the list of images available in the image store
	images, err := s.executor.store.Images()
	if err != nil {
		return "", fmt.Errorf("getting image list from store: %w", err)
	}
	var baseHistory []v1.History
	var baseDiffIDs []digest.Digest
	if s.builder.FromImageID != "" {
		_, baseHistory, baseDiffIDs, err = s.executor.getImageTypeAndHistoryAndDiffIDs(ctx, s.builder.FromImageID)
		if err != nil {
			return "", fmt.Errorf("getting history of base image %q: %w", s.builder.FromImageID, err)
		}
	}
	for _, image := range images {
		// If s.executor.cacheTTL was specified
		// then ignore processing image if it
		// was created before the specified
		// duration.
		if int64(s.executor.cacheTTL) != 0 {
			timeNow := time.Now()
			imageDuration := timeNow.Sub(image.Created)
			if s.executor.cacheTTL < imageDuration {
				continue
			}
		}
		var imageTopLayer *storage.Layer
		var imageParentLayerID string
		if image.TopLayer != "" {
			imageTopLayer, err = s.executor.store.Layer(image.TopLayer)
			if err != nil {
				return "", fmt.Errorf("getting top layer info: %w", err)
			}
			// Figure out which layer from this image we should
			// compare our container's base layer to.
			imageParentLayerID = imageTopLayer.ID
			// If we haven't added a layer here, then our base
			// layer should be the same as the image's layer.  If
			// did add a layer, then our base layer should be the
			// same as the parent of the image's layer.
			if buildAddsLayer {
				imageParentLayerID = imageTopLayer.Parent
			}
		}
		// If the parent of the top layer of an image is equal to the current build image's top layer,
		// it means that this image is potentially a cached intermediate image from a previous
		// build.
		if s.builder.TopLayer != imageParentLayerID {
			continue
		}
		// Next we double check that the history of this image is equivalent to the previous
		// lines in the Dockerfile up till the point we are at in the build.
		manifestType, history, diffIDs, err := s.executor.getImageTypeAndHistoryAndDiffIDs(ctx, image.ID)
		if err != nil {
			// It's possible that this image is for another architecture, which results
			// in a custom-crafted error message that we'd have to use substring matching
			// to recognize.  Instead, ignore the image.
			logrus.Debugf("error getting history of %q (%v), ignoring it", image.ID, err)
			continue
		}
		// If this candidate isn't of the type that we're building, then it may have lost
		// some format-specific information that a building-without-cache run wouldn't lose.
		if manifestType != s.executor.outputFormat {
			continue
		}
		// children + currNode is the point of the Dockerfile we are currently at.
		if s.historyAndDiffIDsMatch(baseHistory, baseDiffIDs, currNode, history, diffIDs, addedContentDigest, buildAddsLayer) {
			return image.ID, nil
		}
	}
	return "", nil
}

// commit writes the container's contents to an image, using a passed-in tag as
// the name if there is one, generating a unique ID-based one otherwise.
// or commit via any custom exporter if specified.
func (s *StageExecutor) commit(ctx context.Context, createdBy string, emptyLayer bool, output string, squash, finalInstruction bool) (string, reference.Canonical, error) {
	ib := s.stage.Builder
	var imageRef types.ImageReference
	if output != "" {
		imageRef2, err := s.executor.resolveNameToImageRef(output)
		if err != nil {
			return "", nil, err
		}
		imageRef = imageRef2
	}

	if ib.Author != "" {
		s.builder.SetMaintainer(ib.Author)
	}
	config := ib.Config()
	if createdBy != "" {
		s.builder.SetCreatedBy(createdBy)
	}
	s.builder.SetHostname(config.Hostname)
	s.builder.SetDomainname(config.Domainname)
	if s.executor.architecture != "" {
		s.builder.SetArchitecture(s.executor.architecture)
	}
	if s.executor.os != "" {
		s.builder.SetOS(s.executor.os)
	}
	if s.executor.osVersion != "" {
		s.builder.SetOSVersion(s.executor.osVersion)
	}
	for _, osFeatureSpec := range s.executor.osFeatures {
		switch {
		case strings.HasSuffix(osFeatureSpec, "-"):
			s.builder.UnsetOSFeature(strings.TrimSuffix(osFeatureSpec, "-"))
		default:
			s.builder.SetOSFeature(osFeatureSpec)
		}
	}
	s.builder.SetUser(config.User)
	s.builder.ClearPorts()
	for p := range config.ExposedPorts {
		s.builder.SetPort(string(p))
	}
	for _, envSpec := range config.Env {
		spec := strings.SplitN(envSpec, "=", 2)
		s.builder.SetEnv(spec[0], spec[1])
	}
	for _, envSpec := range s.executor.unsetEnvs {
		s.builder.UnsetEnv(envSpec)
	}
	s.builder.SetCmd(config.Cmd)
	s.builder.ClearVolumes()
	for v := range config.Volumes {
		s.builder.AddVolume(v)
	}
	s.builder.ClearOnBuild()
	for _, onBuildSpec := range config.OnBuild {
		s.builder.SetOnBuild(onBuildSpec)
	}
	s.builder.SetWorkDir(config.WorkingDir)
	s.builder.SetEntrypoint(config.Entrypoint)
	s.builder.SetShell(config.Shell)
	s.builder.SetStopSignal(config.StopSignal)
	if config.Healthcheck != nil {
		s.builder.SetHealthcheck(&buildahdocker.HealthConfig{
			Test:        append([]string{}, config.Healthcheck.Test...),
			Interval:    config.Healthcheck.Interval,
			Timeout:     config.Healthcheck.Timeout,
			StartPeriod: config.Healthcheck.StartPeriod,
			Retries:     config.Healthcheck.Retries,
		})
	} else {
		s.builder.SetHealthcheck(nil)
	}
	s.builder.ClearLabels()

	if output == "" {
		// If output is not set then we are committing
		// an intermediate image, in such case we must
		// honor layer labels if they are configured.
		for _, labelString := range s.executor.layerLabels {
			label := strings.SplitN(labelString, "=", 2)
			if len(label) > 1 {
				s.builder.SetLabel(label[0], label[1])
			} else {
				s.builder.SetLabel(label[0], "")
			}
		}
	}
	for k, v := range config.Labels {
		s.builder.SetLabel(k, v)
	}
	if s.executor.commonBuildOptions.IdentityLabel == types.OptionalBoolUndefined || s.executor.commonBuildOptions.IdentityLabel == types.OptionalBoolTrue {
		s.builder.SetLabel(buildah.BuilderIdentityAnnotation, define.Version)
	}
	for _, key := range s.executor.unsetLabels {
		s.builder.UnsetLabel(key)
	}
	for _, annotationSpec := range s.executor.annotations {
		annotation := strings.SplitN(annotationSpec, "=", 2)
		if len(annotation) > 1 {
			s.builder.SetAnnotation(annotation[0], annotation[1])
		} else {
			s.builder.SetAnnotation(annotation[0], "")
		}
	}
	if imageRef != nil {
		logName := transports.ImageName(imageRef)
		logrus.Debugf("COMMIT %q", logName)
	} else {
		logrus.Debugf("COMMIT")
	}
	writer := s.executor.reportWriter
	if s.executor.layers || !s.executor.useCache {
		writer = nil
	}
	options := buildah.CommitOptions{
		Compression:           s.executor.compression,
		SignaturePolicyPath:   s.executor.signaturePolicyPath,
		ReportWriter:          writer,
		PreferredManifestType: s.executor.outputFormat,
		SystemContext:         s.executor.systemContext,
		Squash:                squash,
		OmitHistory:           s.executor.commonBuildOptions.OmitHistory,
		EmptyLayer:            emptyLayer,
		BlobDirectory:         s.executor.blobDirectory,
		SignBy:                s.executor.signBy,
		MaxRetries:            s.executor.maxPullPushRetries,
		RetryDelay:            s.executor.retryPullPushDelay,
		HistoryTimestamp:      s.executor.timestamp,
		Manifest:              s.executor.manifest,
	}
	if finalInstruction {
		options.ConfidentialWorkloadOptions = s.executor.confidentialWorkload
	}
	imgID, _, manifestDigest, err := s.builder.Commit(ctx, imageRef, options)
	if err != nil {
		return "", nil, err
	}
	var ref reference.Canonical
	if imageRef != nil {
		if dref := imageRef.DockerReference(); dref != nil {
			if ref, err = reference.WithDigest(dref, manifestDigest); err != nil {
				return "", nil, fmt.Errorf("computing canonical reference for new image %q: %w", imgID, err)
			}
		}
	}
	return imgID, ref, nil
}

func (s *StageExecutor) generateBuildOutput(buildOutputOpts define.BuildOutputOption) error {
	extractRootfsOpts := buildah.ExtractRootfsOptions{}
	if unshare.IsRootless() {
		// In order to maintain as much parity as possible
		// with buildkit's version of --output and to avoid
		// unsafe invocation of exported executables it was
		// decided to strip setuid,setgid and extended attributes.
		// Since modes like setuid,setgid leaves room for executable
		// to get invoked with different file-system permission its safer
		// to strip them off for unprivileged invocation.
		// See: https://github.com/containers/buildah/pull/3823#discussion_r829376633
		extractRootfsOpts.StripSetuidBit = true
		extractRootfsOpts.StripSetgidBit = true
		extractRootfsOpts.StripXattrs = true
	}
	rc, errChan, err := s.builder.ExtractRootfs(buildah.CommitOptions{}, extractRootfsOpts)
	if err != nil {
		return fmt.Errorf("failed to extract rootfs from given container image: %w", err)
	}
	defer rc.Close()
	err = internalUtil.ExportFromReader(rc, buildOutputOpts)
	if err != nil {
		return fmt.Errorf("failed to export build output: %w", err)
	}
	if errChan != nil {
		err = <-errChan
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *StageExecutor) EnsureContainerPath(path string) error {
	return s.builder.EnsureContainerPathAs(path, "", nil)
}

func (s *StageExecutor) EnsureContainerPathAs(path, user string, mode *os.FileMode) error {
	return s.builder.EnsureContainerPathAs(path, user, mode)
}
