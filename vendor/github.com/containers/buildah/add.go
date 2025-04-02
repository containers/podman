package buildah

import (
	"archive/tar"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/containers/buildah/copier"
	"github.com/containers/buildah/define"
	"github.com/containers/buildah/internal/tmpdir"
	"github.com/containers/buildah/pkg/chrootuser"
	"github.com/containers/common/pkg/retry"
	"github.com/containers/image/v5/pkg/tlsclientconfig"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage/pkg/fileutils"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/regexp"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/hashicorp/go-multierror"
	"github.com/moby/sys/userns"
	digest "github.com/opencontainers/go-digest"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

// AddAndCopyOptions holds options for add and copy commands.
type AddAndCopyOptions struct {
	// Chmod sets the access permissions of the destination content.
	Chmod string
	// Chown is a spec for the user who should be given ownership over the
	// newly-added content, potentially overriding permissions which would
	// otherwise be set to 0:0.
	Chown string
	// Checksum is a standard container digest string (e.g. <algorithm>:<digest>)
	// and is the expected hash of the content being copied.
	Checksum string
	// PreserveOwnership, if Chown is not set, tells us to avoid setting
	// ownership of copied items to 0:0, instead using whatever ownership
	// information is already set.  Not meaningful for remote sources or
	// local archives that we extract.
	PreserveOwnership bool
	// All of the data being copied will pass through Hasher, if set.
	// If the sources are URLs or files, their contents will be passed to
	// Hasher.
	// If the sources include directory trees, Hasher will be passed
	// tar-format archives of the directory trees.
	Hasher io.Writer
	// Excludes is the contents of the .containerignore file.
	Excludes []string
	// IgnoreFile is the path to the .containerignore file.
	IgnoreFile string
	// ContextDir is the base directory for content being copied and
	// Excludes patterns.
	ContextDir string
	// ID mapping options to use when contents to be copied are part of
	// another container, and need ownerships to be mapped from the host to
	// that container's values before copying them into the container.
	IDMappingOptions *define.IDMappingOptions
	// DryRun indicates that the content should be digested, but not actually
	// copied into the container.
	DryRun bool
	// Clear the setuid bit on items being copied.  Has no effect on
	// archives being extracted, where the bit is always preserved.
	StripSetuidBit bool
	// Clear the setgid bit on items being copied.  Has no effect on
	// archives being extracted, where the bit is always preserved.
	StripSetgidBit bool
	// Clear the sticky bit on items being copied.  Has no effect on
	// archives being extracted, where the bit is always preserved.
	StripStickyBit bool
	// If not "", a directory containing a CA certificate (ending with
	// ".crt"), a client certificate (ending with ".cert") and a client
	// certificate key (ending with ".key") used when downloading sources
	// from locations protected with TLS.
	CertPath string
	// Allow downloading sources from HTTPS where TLS verification fails.
	InsecureSkipTLSVerify types.OptionalBool
	// MaxRetries is the maximum number of attempts we'll make to retrieve
	// contents from a remote location.
	MaxRetries int
	// RetryDelay is how long to wait before retrying attempts to retrieve
	// remote contents.
	RetryDelay time.Duration
}

// gitURLFragmentSuffix matches fragments to use as Git reference and build
// context from the Git repository e.g.
//
//	github.com/containers/buildah.git
//	github.com/containers/buildah.git#main
//	github.com/containers/buildah.git#v1.35.0
var gitURLFragmentSuffix = regexp.Delayed(`\.git(?:#.+)?$`)

// sourceIsGit returns true if "source" is a git location.
func sourceIsGit(source string) bool {
	return isURL(source) && gitURLFragmentSuffix.MatchString(source)
}

func isURL(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

// sourceIsRemote returns true if "source" is a remote location
// and *not* a git repo. Certain github urls such as raw.github.* are allowed.
func sourceIsRemote(source string) bool {
	return isURL(source) && !gitURLFragmentSuffix.MatchString(source)
}

// getURL writes a tar archive containing the named content
func getURL(src string, chown *idtools.IDPair, mountpoint, renameTarget string, writer io.Writer, chmod *os.FileMode, srcDigest digest.Digest, certPath string, insecureSkipTLSVerify types.OptionalBool) error {
	url, err := url.Parse(src)
	if err != nil {
		return err
	}
	tlsClientConfig := &tls.Config{
		CipherSuites: tlsconfig.DefaultServerAcceptedCiphers,
	}
	if err := tlsclientconfig.SetupCertificates(certPath, tlsClientConfig); err != nil {
		return err
	}
	tlsClientConfig.InsecureSkipVerify = insecureSkipTLSVerify == types.OptionalBoolTrue

	tr := &http.Transport{TLSClientConfig: tlsClientConfig}
	httpClient := &http.Client{Transport: tr}
	response, err := httpClient.Get(src)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("invalid response status %d", response.StatusCode)
	}

	// Figure out what to name the new content.
	name := renameTarget
	if name == "" {
		name = path.Base(url.Path)
	}
	// If there's a date on the content, use it.  If not, use the Unix epoch
	// for compatibility.
	date := time.Unix(0, 0).UTC()
	lastModified := response.Header.Get("Last-Modified")
	if lastModified != "" {
		d, err := time.Parse(time.RFC1123, lastModified)
		if err != nil {
			return fmt.Errorf("parsing last-modified time: %w", err)
		}
		date = d
	}
	// Figure out the size of the content.
	size := response.ContentLength
	var responseBody io.Reader = response.Body
	if size < 0 {
		// Create a temporary file and copy the content to it, so that
		// we can figure out how much content there is.
		f, err := os.CreateTemp(mountpoint, "download")
		if err != nil {
			return fmt.Errorf("creating temporary file to hold %q: %w", src, err)
		}
		defer os.Remove(f.Name())
		defer f.Close()
		size, err = io.Copy(f, response.Body)
		if err != nil {
			return fmt.Errorf("writing %q to temporary file %q: %w", src, f.Name(), err)
		}
		_, err = f.Seek(0, io.SeekStart)
		if err != nil {
			return fmt.Errorf("setting up to read %q from temporary file %q: %w", src, f.Name(), err)
		}
		responseBody = f
	}
	var digester digest.Digester
	if srcDigest != "" {
		digester = srcDigest.Algorithm().Digester()
		responseBody = io.TeeReader(responseBody, digester.Hash())
	}
	// Write the output archive.  Set permissions for compatibility.
	tw := tar.NewWriter(writer)
	defer tw.Close()
	uid := 0
	gid := 0
	if chown != nil {
		uid = chown.UID
		gid = chown.GID
	}
	var mode int64 = 0o600
	if chmod != nil {
		mode = int64(*chmod)
	}
	hdr := tar.Header{
		Typeflag: tar.TypeReg,
		Name:     name,
		Size:     size,
		Uid:      uid,
		Gid:      gid,
		Mode:     mode,
		ModTime:  date,
	}
	err = tw.WriteHeader(&hdr)
	if err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	if _, err := io.Copy(tw, responseBody); err != nil {
		return fmt.Errorf("writing content from %q to tar stream: %w", src, err)
	}

	if digester != nil {
		if responseDigest := digester.Digest(); responseDigest != srcDigest {
			return fmt.Errorf("unexpected response digest for %q: %s, want %s", src, responseDigest, srcDigest)
		}
	}

	return nil
}

// includeDirectoryAnyway returns true if "path" is a prefix for an exception
// known to "pm".  If "path" is a directory that "pm" claims matches its list
// of patterns, but "pm"'s list of exclusions contains a pattern for which
// "path" is a prefix, then IncludeDirectoryAnyway() will return true.
// This is not always correct, because it relies on the directory part of any
// exception paths to be specified without wildcards.
func includeDirectoryAnyway(path string, pm *fileutils.PatternMatcher) bool {
	if !pm.Exclusions() {
		return false
	}
	prefix := strings.TrimPrefix(path, string(os.PathSeparator)) + string(os.PathSeparator)
	for _, pattern := range pm.Patterns() {
		if !pattern.Exclusion() {
			continue
		}
		spec := strings.TrimPrefix(pattern.String(), string(os.PathSeparator))
		if strings.HasPrefix(spec, prefix) {
			return true
		}
	}
	return false
}

// globbedToGlobbable takes a pathname which might include the '[', *, or ?
// characters, and converts it into a glob pattern that matches itself by
// marking the '[' characters as _not_ the beginning of match ranges and
// escaping the * and ? characters.
func globbedToGlobbable(glob string) string {
	result := glob
	result = strings.ReplaceAll(result, "[", "[[]")
	result = strings.ReplaceAll(result, "?", "\\?")
	result = strings.ReplaceAll(result, "*", "\\*")
	return result
}

// Add copies the contents of the specified sources into the container's root
// filesystem, optionally extracting contents of local files that look like
// non-empty archives.
func (b *Builder) Add(destination string, extract bool, options AddAndCopyOptions, sources ...string) error {
	mountPoint, err := b.Mount(b.MountLabel)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := b.Unmount(); err2 != nil {
			logrus.Errorf("error unmounting container: %v", err2)
		}
	}()

	contextDir := options.ContextDir
	currentDir := options.ContextDir
	if options.ContextDir == "" {
		contextDir = string(os.PathSeparator)
		currentDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("determining current working directory: %w", err)
		}
	} else {
		if !filepath.IsAbs(options.ContextDir) {
			contextDir, err = filepath.Abs(options.ContextDir)
			if err != nil {
				return fmt.Errorf("converting context directory path %q to an absolute path: %w", options.ContextDir, err)
			}
		}
	}

	// Figure out what sorts of sources we have.
	var localSources, remoteSources, gitSources []string
	for i, src := range sources {
		if src == "" {
			return errors.New("empty source location")
		}
		if sourceIsRemote(src) {
			remoteSources = append(remoteSources, src)
			continue
		}
		if sourceIsGit(src) {
			gitSources = append(gitSources, src)
			continue
		}
		if !filepath.IsAbs(src) && options.ContextDir == "" {
			sources[i] = filepath.Join(currentDir, src)
		}
		localSources = append(localSources, sources[i])
	}

	// Treat git sources as a subset of remote sources
	// differentiating only in how we fetch the two later on.
	if len(gitSources) > 0 {
		remoteSources = append(remoteSources, gitSources...)
	}

	// Check how many items our local source specs matched.  Each spec
	// should have matched at least one item, otherwise we consider it an
	// error.
	var localSourceStats []*copier.StatsForGlob
	if len(localSources) > 0 {
		statOptions := copier.StatOptions{
			CheckForArchives: extract,
		}
		localSourceStats, err = copier.Stat(contextDir, contextDir, statOptions, localSources)
		if err != nil {
			return fmt.Errorf("checking on sources under %q: %w", contextDir, err)
		}
	}
	numLocalSourceItems := 0
	for _, localSourceStat := range localSourceStats {
		if localSourceStat.Error != "" {
			errorText := localSourceStat.Error
			rel, err := filepath.Rel(contextDir, localSourceStat.Glob)
			if err != nil {
				errorText = fmt.Sprintf("%v; %s", err, errorText)
			}
			if strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
				errorText = fmt.Sprintf("possible escaping context directory error: %s", errorText)
			}
			return fmt.Errorf("checking on sources under %q: %v", contextDir, errorText)
		}
		if len(localSourceStat.Globbed) == 0 {
			return fmt.Errorf("checking source under %q: no glob matches: %w", contextDir, syscall.ENOENT)
		}
		numLocalSourceItems += len(localSourceStat.Globbed)
	}
	if numLocalSourceItems+len(remoteSources)+len(gitSources) == 0 {
		return fmt.Errorf("no sources %v found: %w", sources, syscall.ENOENT)
	}

	// Find out which user (and group) the destination should belong to.
	var chownDirs, chownFiles *idtools.IDPair
	var userUID, userGID uint32
	if options.Chown != "" {
		userUID, userGID, err = b.userForCopy(mountPoint, options.Chown)
		if err != nil {
			return fmt.Errorf("looking up UID/GID for %q: %w", options.Chown, err)
		}
	}
	var chmodDirsFiles *os.FileMode
	if options.Chmod != "" {
		p, err := strconv.ParseUint(options.Chmod, 8, 32)
		if err != nil {
			return fmt.Errorf("parsing chmod %q: %w", options.Chmod, err)
		}
		perm := os.FileMode(p)
		chmodDirsFiles = &perm
	}

	chownDirs = &idtools.IDPair{UID: int(userUID), GID: int(userGID)}
	chownFiles = &idtools.IDPair{UID: int(userUID), GID: int(userGID)}
	if options.Chown == "" && options.PreserveOwnership {
		chownDirs = nil
		chownFiles = nil
	}

	// If we have a single source archive to extract, or more than one
	// source item, or the destination has a path separator at the end of
	// it, and it's not a remote URL, the destination needs to be a
	// directory.
	if destination == "" || !filepath.IsAbs(destination) {
		tmpDestination := filepath.Join(string(os.PathSeparator)+b.WorkDir(), destination)
		if destination == "" || strings.HasSuffix(destination, string(os.PathSeparator)) {
			destination = tmpDestination + string(os.PathSeparator)
		} else {
			destination = tmpDestination
		}
	}
	destMustBeDirectory := (len(sources) > 1) || strings.HasSuffix(destination, string(os.PathSeparator)) || destination == b.WorkDir()
	destCanBeFile := false
	if len(sources) == 1 {
		if len(remoteSources) == 1 {
			destCanBeFile = sourceIsRemote(sources[0])
		}
		if len(localSources) == 1 {
			item := localSourceStats[0].Results[localSourceStats[0].Globbed[0]]
			if item.IsDir || (item.IsArchive && extract) {
				destMustBeDirectory = true
			}
			if item.IsRegular {
				destCanBeFile = true
			}
		}
		if len(gitSources) > 0 {
			destMustBeDirectory = true
		}
	}

	// We care if the destination either doesn't exist, or exists and is a
	// file.  If the source can be a single file, for those cases we treat
	// the destination as a file rather than as a directory tree.
	renameTarget := ""
	extractDirectory := filepath.Join(mountPoint, destination)
	statOptions := copier.StatOptions{
		CheckForArchives: extract,
	}
	destStats, err := copier.Stat(mountPoint, filepath.Join(mountPoint, b.WorkDir()), statOptions, []string{extractDirectory})
	if err != nil {
		return fmt.Errorf("checking on destination %v: %w", extractDirectory, err)
	}
	if (len(destStats) == 0 || len(destStats[0].Globbed) == 0) && !destMustBeDirectory && destCanBeFile {
		// destination doesn't exist - extract to parent and rename the incoming file to the destination's name
		renameTarget = filepath.Base(extractDirectory)
		extractDirectory = filepath.Dir(extractDirectory)
	}

	// if the destination is a directory that doesn't yet exist, let's copy it.
	newDestDirFound := false
	if (len(destStats) == 1 || len(destStats[0].Globbed) == 0) && destMustBeDirectory && !destCanBeFile {
		newDestDirFound = true
	}

	if len(destStats) == 1 && len(destStats[0].Globbed) == 1 && destStats[0].Results[destStats[0].Globbed[0]].IsRegular {
		if destMustBeDirectory {
			return fmt.Errorf("destination %v already exists but is not a directory", destination)
		}
		// destination exists - it's a file, we need to extract to parent and rename the incoming file to the destination's name
		renameTarget = filepath.Base(extractDirectory)
		extractDirectory = filepath.Dir(extractDirectory)
	}

	pm, err := fileutils.NewPatternMatcher(options.Excludes)
	if err != nil {
		return fmt.Errorf("processing excludes list %v: %w", options.Excludes, err)
	}

	// Make sure that, if it's a symlink, we'll chroot to the target of the link;
	// knowing that target requires that we resolve it within the chroot.
	evalOptions := copier.EvalOptions{}
	evaluated, err := copier.Eval(mountPoint, extractDirectory, evalOptions)
	if err != nil {
		return fmt.Errorf("checking on destination %v: %w", extractDirectory, err)
	}
	extractDirectory = evaluated

	// Set up ID maps.
	var srcUIDMap, srcGIDMap []idtools.IDMap
	if options.IDMappingOptions != nil {
		srcUIDMap, srcGIDMap = convertRuntimeIDMaps(options.IDMappingOptions.UIDMap, options.IDMappingOptions.GIDMap)
	}
	destUIDMap, destGIDMap := convertRuntimeIDMaps(b.IDMappingOptions.UIDMap, b.IDMappingOptions.GIDMap)

	// Create the target directory if it doesn't exist yet.
	mkdirOptions := copier.MkdirOptions{
		UIDMap:   destUIDMap,
		GIDMap:   destGIDMap,
		ChownNew: chownDirs,
	}
	if err := copier.Mkdir(mountPoint, extractDirectory, mkdirOptions); err != nil {
		return fmt.Errorf("ensuring target directory exists: %w", err)
	}

	// Copy each source in turn.
	for _, src := range sources {
		var multiErr *multierror.Error
		var getErr, closeErr, renameErr, putErr error
		var wg sync.WaitGroup
		if sourceIsRemote(src) || sourceIsGit(src) {
			pipeReader, pipeWriter := io.Pipe()
			var srcDigest digest.Digest
			if options.Checksum != "" {
				srcDigest, err = digest.Parse(options.Checksum)
				if err != nil {
					return fmt.Errorf("invalid checksum flag: %w", err)
				}
			}

			wg.Add(1)
			if sourceIsGit(src) {
				go func() {
					var cloneDir, subdir string
					cloneDir, subdir, getErr = define.TempDirForURL(tmpdir.GetTempDir(), "", src)
					getOptions := copier.GetOptions{
						UIDMap:         srcUIDMap,
						GIDMap:         srcGIDMap,
						Excludes:       options.Excludes,
						ExpandArchives: extract,
						ChownDirs:      chownDirs,
						ChmodDirs:      chmodDirsFiles,
						ChownFiles:     chownFiles,
						ChmodFiles:     chmodDirsFiles,
						StripSetuidBit: options.StripSetuidBit,
						StripSetgidBit: options.StripSetgidBit,
						StripStickyBit: options.StripStickyBit,
					}
					writer := io.WriteCloser(pipeWriter)
					repositoryDir := filepath.Join(cloneDir, subdir)
					getErr = copier.Get(repositoryDir, repositoryDir, getOptions, []string{"."}, writer)
					pipeWriter.Close()
					wg.Done()
				}()
			} else {
				go func() {
					getErr = retry.IfNecessary(context.TODO(), func() error {
						return getURL(src, chownFiles, mountPoint, renameTarget, pipeWriter, chmodDirsFiles, srcDigest, options.CertPath, options.InsecureSkipTLSVerify)
					}, &retry.Options{
						MaxRetry: options.MaxRetries,
						Delay:    options.RetryDelay,
					})
					pipeWriter.Close()
					wg.Done()
				}()
			}

			wg.Add(1)
			go func() {
				b.ContentDigester.Start("")
				hashCloser := b.ContentDigester.Hash()
				hasher := io.Writer(hashCloser)
				if options.Hasher != nil {
					hasher = io.MultiWriter(hasher, options.Hasher)
				}
				if options.DryRun {
					_, putErr = io.Copy(hasher, pipeReader)
				} else {
					putOptions := copier.PutOptions{
						UIDMap:        destUIDMap,
						GIDMap:        destGIDMap,
						ChownDirs:     nil,
						ChmodDirs:     nil,
						ChownFiles:    nil,
						ChmodFiles:    nil,
						IgnoreDevices: userns.RunningInUserNS(),
					}
					putErr = copier.Put(extractDirectory, extractDirectory, putOptions, io.TeeReader(pipeReader, hasher))
				}
				hashCloser.Close()
				pipeReader.Close()
				wg.Done()
			}()
			wg.Wait()
			if getErr != nil {
				getErr = fmt.Errorf("reading %q: %w", src, getErr)
			}
			if putErr != nil {
				putErr = fmt.Errorf("storing %q: %w", src, putErr)
			}
			multiErr = multierror.Append(getErr, putErr)
			if multiErr != nil && multiErr.ErrorOrNil() != nil {
				if len(multiErr.Errors) > 1 {
					return multiErr.ErrorOrNil()
				}
				return multiErr.Errors[0]
			}
			continue
		}

		if options.Checksum != "" {
			return fmt.Errorf("checksum flag is not supported for local sources")
		}

		// Dig out the result of running glob+stat on this source spec.
		var localSourceStat *copier.StatsForGlob
		for _, st := range localSourceStats {
			if st.Glob == src {
				localSourceStat = st
				break
			}
		}
		if localSourceStat == nil {
			continue
		}

		// Iterate through every item that matched the glob.
		itemsCopied := 0
		for _, globbed := range localSourceStat.Globbed {
			rel := globbed
			if filepath.IsAbs(globbed) {
				if rel, err = filepath.Rel(contextDir, globbed); err != nil {
					return fmt.Errorf("computing path of %q relative to %q: %w", globbed, contextDir, err)
				}
			}
			if strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
				return fmt.Errorf("possible escaping context directory error: %q is outside of %q", globbed, contextDir)
			}
			// Check for dockerignore-style exclusion of this item.
			if rel != "." {
				excluded, err := pm.Matches(filepath.ToSlash(rel)) // nolint:staticcheck
				if err != nil {
					return fmt.Errorf("checking if %q(%q) is excluded: %w", globbed, rel, err)
				}
				if excluded {
					// non-directories that are excluded are excluded, no question, but
					// directories can only be skipped if we don't have to allow for the
					// possibility of finding things to include under them
					globInfo := localSourceStat.Results[globbed]
					if !globInfo.IsDir || !includeDirectoryAnyway(rel, pm) {
						continue
					}
				} else {
					// if the destination is a directory that doesn't yet exist, and is not excluded, let's copy it.
					if newDestDirFound {
						itemsCopied++
					}
				}
			} else {
				// Make sure we don't trigger a "copied nothing" error for an empty context
				// directory if we were told to copy the context directory itself.  We won't
				// actually copy it, but we need to make sure that we don't produce an error
				// due to potentially not having anything in the tarstream that we passed.
				itemsCopied++
			}
			st := localSourceStat.Results[globbed]
			pipeReader, pipeWriter := io.Pipe()
			wg.Add(1)
			go func() {
				renamedItems := 0
				writer := io.WriteCloser(pipeWriter)
				if renameTarget != "" {
					writer = newTarFilterer(writer, func(hdr *tar.Header) (bool, bool, io.Reader) {
						hdr.Name = renameTarget
						renamedItems++
						return false, false, nil
					})
				}
				writer = newTarFilterer(writer, func(_ *tar.Header) (bool, bool, io.Reader) {
					itemsCopied++
					return false, false, nil
				})
				getOptions := copier.GetOptions{
					UIDMap:         srcUIDMap,
					GIDMap:         srcGIDMap,
					Excludes:       options.Excludes,
					ExpandArchives: extract,
					ChownDirs:      chownDirs,
					ChmodDirs:      chmodDirsFiles,
					ChownFiles:     chownFiles,
					ChmodFiles:     chmodDirsFiles,
					StripSetuidBit: options.StripSetuidBit,
					StripSetgidBit: options.StripSetgidBit,
					StripStickyBit: options.StripStickyBit,
				}
				getErr = copier.Get(contextDir, contextDir, getOptions, []string{globbedToGlobbable(globbed)}, writer)
				closeErr = writer.Close()
				if renameTarget != "" && renamedItems > 1 {
					renameErr = fmt.Errorf("internal error: renamed %d items when we expected to only rename 1", renamedItems)
				}
				wg.Done()
			}()
			wg.Add(1)
			go func() {
				if st.IsDir {
					b.ContentDigester.Start("dir")
				} else {
					b.ContentDigester.Start("file")
				}
				hashCloser := b.ContentDigester.Hash()
				hasher := io.Writer(hashCloser)
				if options.Hasher != nil {
					hasher = io.MultiWriter(hasher, options.Hasher)
				}
				if options.DryRun {
					_, putErr = io.Copy(hasher, pipeReader)
				} else {
					putOptions := copier.PutOptions{
						UIDMap:          destUIDMap,
						GIDMap:          destGIDMap,
						DefaultDirOwner: chownDirs,
						DefaultDirMode:  nil,
						ChownDirs:       nil,
						ChmodDirs:       nil,
						ChownFiles:      nil,
						ChmodFiles:      nil,
						IgnoreDevices:   userns.RunningInUserNS(),
					}
					putErr = copier.Put(extractDirectory, extractDirectory, putOptions, io.TeeReader(pipeReader, hasher))
				}
				hashCloser.Close()
				pipeReader.Close()
				wg.Done()
			}()
			wg.Wait()
			if getErr != nil {
				getErr = fmt.Errorf("reading %q: %w", src, getErr)
			}
			if closeErr != nil {
				closeErr = fmt.Errorf("closing %q: %w", src, closeErr)
			}
			if renameErr != nil {
				renameErr = fmt.Errorf("renaming %q: %w", src, renameErr)
			}
			if putErr != nil {
				putErr = fmt.Errorf("storing %q: %w", src, putErr)
			}
			multiErr = multierror.Append(getErr, closeErr, renameErr, putErr)
			if multiErr != nil && multiErr.ErrorOrNil() != nil {
				if len(multiErr.Errors) > 1 {
					return multiErr.ErrorOrNil()
				}
				return multiErr.Errors[0]
			}
		}
		if itemsCopied == 0 {
			excludesFile := ""
			if options.IgnoreFile != "" {
				excludesFile = " using " + options.IgnoreFile
			}
			return fmt.Errorf("no items matching glob %q copied (%d filtered out%s): %w", localSourceStat.Glob, len(localSourceStat.Globbed), excludesFile, syscall.ENOENT)
		}
	}
	return nil
}

// userForRun returns the user (and group) information which we should use for
// running commands
func (b *Builder) userForRun(mountPoint string, userspec string) (specs.User, string, error) {
	if userspec == "" {
		userspec = b.User()
	}

	uid, gid, homeDir, err := chrootuser.GetUser(mountPoint, userspec)
	u := specs.User{
		UID:      uid,
		GID:      gid,
		Username: userspec,
	}
	if !strings.Contains(userspec, ":") {
		groups, err2 := chrootuser.GetAdditionalGroupsForUser(mountPoint, uint64(u.UID))
		if err2 != nil {
			if !errors.Is(err2, chrootuser.ErrNoSuchUser) && err == nil {
				err = err2
			}
		} else {
			u.AdditionalGids = groups
		}
	}
	return u, homeDir, err
}

// userForCopy returns the user (and group) information which we should use for
// setting ownership of contents being copied.  It's just like what
// userForRun() does, except for the case where we're passed a single numeric
// value, where we need to use that value for both the UID and the GID.
func (b *Builder) userForCopy(mountPoint string, userspec string) (uint32, uint32, error) {
	var (
		user, group string
		uid, gid    uint64
		err         error
	)

	split := strings.SplitN(userspec, ":", 2)
	user = split[0]
	if len(split) > 1 {
		group = split[1]
	}

	// If userspec did not specify any values for user or group, then fail
	if user == "" && group == "" {
		return 0, 0, fmt.Errorf("can't find uid for user %s", userspec)
	}

	// If userspec specifies values for user or group, check for numeric values
	// and return early.  If not, then translate username/groupname
	if user != "" {
		uid, err = strconv.ParseUint(user, 10, 32)
	}
	if err == nil {
		// default gid to uid
		gid = uid
		if group != "" {
			gid, err = strconv.ParseUint(group, 10, 32)
		}
	}
	// If err != nil, then user or group not numeric, check filesystem
	if err == nil {
		return uint32(uid), uint32(gid), nil
	}

	owner, _, err := b.userForRun(mountPoint, userspec)
	if err != nil {
		return 0xffffffff, 0xffffffff, err
	}
	return owner.UID, owner.GID, nil
}

// EnsureContainerPathAs creates the specified directory if it doesn't exist,
// setting a newly-created directory's owner to USER and its permissions to MODE.
func (b *Builder) EnsureContainerPathAs(path, user string, mode *os.FileMode) error {
	mountPoint, err := b.Mount(b.MountLabel)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := b.Unmount(); err2 != nil {
			logrus.Errorf("error unmounting container: %v", err2)
		}
	}()

	uid, gid := uint32(0), uint32(0)
	if user != "" {
		if uidForCopy, gidForCopy, err := b.userForCopy(mountPoint, user); err == nil {
			uid = uidForCopy
			gid = gidForCopy
		}
	}

	destUIDMap, destGIDMap := convertRuntimeIDMaps(b.IDMappingOptions.UIDMap, b.IDMappingOptions.GIDMap)

	idPair := &idtools.IDPair{UID: int(uid), GID: int(gid)}
	opts := copier.MkdirOptions{
		ChmodNew: mode,
		ChownNew: idPair,
		UIDMap:   destUIDMap,
		GIDMap:   destGIDMap,
	}
	return copier.Mkdir(mountPoint, filepath.Join(mountPoint, path), opts)
}
