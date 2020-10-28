package abi

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/buildah/pkg/chrootuser"
	"github.com/containers/buildah/util"
	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/chrootarchive"
	"github.com/containers/storage/pkg/idtools"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/docker/docker/pkg/archive"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func (ic *ContainerEngine) ContainerCp(ctx context.Context, source, dest string, options entities.ContainerCpOptions) (*entities.ContainerCpReport, error) {
	extract := options.Extract

	srcCtr, srcPath := parsePath(ic.Libpod, source)
	destCtr, destPath := parsePath(ic.Libpod, dest)

	if (srcCtr == nil && destCtr == nil) || (srcCtr != nil && destCtr != nil) {
		return nil, errors.Errorf("invalid arguments %s, %s you must use just one container", source, dest)
	}

	if len(srcPath) == 0 || len(destPath) == 0 {
		return nil, errors.Errorf("invalid arguments %s, %s you must specify paths", source, dest)
	}
	ctr := srcCtr
	isFromHostToCtr := ctr == nil
	if isFromHostToCtr {
		ctr = destCtr
	}

	mountPoint, err := ctr.Mount()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := ctr.Unmount(false); err != nil {
			logrus.Errorf("unable to umount container '%s': %q", ctr.ID(), err)
		}
	}()

	if options.Pause {
		if err := ctr.Pause(); err != nil {
			// An invalid state error is fine.
			// The container isn't running or is already paused.
			// TODO: We can potentially start the container while
			// the copy is running, which still allows a race where
			// malicious code could mess with the symlink.
			if errors.Cause(err) != define.ErrCtrStateInvalid {
				return nil, err
			}
		} else {
			// Only add the defer if we actually paused
			defer func() {
				if err := ctr.Unpause(); err != nil {
					logrus.Errorf("Error unpausing container after copying: %v", err)
				}
			}()
		}
	}

	user, err := getUser(mountPoint, ctr.User())
	if err != nil {
		return nil, err
	}
	idMappingOpts, err := ctr.IDMappings()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting IDMappingOptions")
	}
	destOwner := idtools.IDPair{UID: int(user.UID), GID: int(user.GID)}
	hostUID, hostGID, err := util.GetHostIDs(convertIDMap(idMappingOpts.UIDMap), convertIDMap(idMappingOpts.GIDMap), user.UID, user.GID)
	if err != nil {
		return nil, err
	}

	hostOwner := idtools.IDPair{UID: int(hostUID), GID: int(hostGID)}

	if isFromHostToCtr {
		if isVol, volDestName, volName := isVolumeDestName(destPath, ctr); isVol { //nolint(gocritic)
			path, err := pathWithVolumeMount(ic.Libpod, volDestName, volName, destPath)
			if err != nil {
				return nil, errors.Wrapf(err, "error getting destination path from volume %s", volDestName)
			}
			destPath = path
		} else if isBindMount, mount := isBindMountDestName(destPath, ctr); isBindMount { //nolint(gocritic)
			path, err := pathWithBindMountSource(mount, destPath)
			if err != nil {
				return nil, errors.Wrapf(err, "error getting destination path from bind mount %s", mount.Destination)
			}
			destPath = path
		} else if filepath.IsAbs(destPath) { //nolint(gocritic)
			cleanedPath, err := securejoin.SecureJoin(mountPoint, destPath)
			if err != nil {
				return nil, err
			}
			destPath = cleanedPath
		} else { //nolint(gocritic)
			ctrWorkDir, err := securejoin.SecureJoin(mountPoint, ctr.WorkingDir())
			if err != nil {
				return nil, err
			}
			if err = idtools.MkdirAllAndChownNew(ctrWorkDir, 0755, hostOwner); err != nil {
				return nil, err
			}
			cleanedPath, err := securejoin.SecureJoin(mountPoint, filepath.Join(ctr.WorkingDir(), destPath))
			if err != nil {
				return nil, err
			}
			destPath = cleanedPath
		}
	} else {
		destOwner = idtools.IDPair{UID: os.Getuid(), GID: os.Getgid()}
		if isVol, volDestName, volName := isVolumeDestName(srcPath, ctr); isVol { //nolint(gocritic)
			path, err := pathWithVolumeMount(ic.Libpod, volDestName, volName, srcPath)
			if err != nil {
				return nil, errors.Wrapf(err, "error getting source path from volume %s", volDestName)
			}
			srcPath = path
		} else if isBindMount, mount := isBindMountDestName(srcPath, ctr); isBindMount { //nolint(gocritic)
			path, err := pathWithBindMountSource(mount, srcPath)
			if err != nil {
				return nil, errors.Wrapf(err, "error getting source path from bind mount %s", mount.Destination)
			}
			srcPath = path
		} else if filepath.IsAbs(srcPath) { //nolint(gocritic)
			cleanedPath, err := securejoin.SecureJoin(mountPoint, srcPath)
			if err != nil {
				return nil, err
			}
			srcPath = cleanedPath
		} else { //nolint(gocritic)
			cleanedPath, err := securejoin.SecureJoin(mountPoint, filepath.Join(ctr.WorkingDir(), srcPath))
			if err != nil {
				return nil, err
			}
			srcPath = cleanedPath
		}
	}

	if !filepath.IsAbs(destPath) {
		dir, err := os.Getwd()
		if err != nil {
			return nil, errors.Wrapf(err, "err getting current working directory")
		}
		destPath = filepath.Join(dir, destPath)
	}

	if source == "-" {
		srcPath = os.Stdin.Name()
		extract = true
	}
	err = containerCopy(srcPath, destPath, source, dest, idMappingOpts, &destOwner, extract, isFromHostToCtr)
	return &entities.ContainerCpReport{}, err
}

func getUser(mountPoint string, userspec string) (specs.User, error) {
	uid, gid, _, err := chrootuser.GetUser(mountPoint, userspec)
	u := specs.User{
		UID:      uid,
		GID:      gid,
		Username: userspec,
	}
	if !strings.Contains(userspec, ":") {
		groups, err2 := chrootuser.GetAdditionalGroupsForUser(mountPoint, uint64(u.UID))
		if err2 != nil {
			if errors.Cause(err2) != chrootuser.ErrNoSuchUser && err == nil {
				err = err2
			}
		} else {
			u.AdditionalGids = groups
		}

	}
	return u, err
}

func parsePath(runtime *libpod.Runtime, path string) (*libpod.Container, string) {
	pathArr := strings.SplitN(path, ":", 2)
	if len(pathArr) == 2 {
		ctr, err := runtime.LookupContainer(pathArr[0])
		if err == nil {
			return ctr, pathArr[1]
		}
	}
	return nil, path
}

func evalSymlinks(path string) (string, error) {
	if path == os.Stdin.Name() {
		return path, nil
	}
	return filepath.EvalSymlinks(path)
}

func getPathInfo(path string) (string, os.FileInfo, error) {
	path, err := evalSymlinks(path)
	if err != nil {
		return "", nil, errors.Wrapf(err, "error evaluating symlinks %q", path)
	}
	srcfi, err := os.Stat(path)
	if err != nil {
		return "", nil, err
	}
	return path, srcfi, nil
}

func containerCopy(srcPath, destPath, src, dest string, idMappingOpts storage.IDMappingOptions, chownOpts *idtools.IDPair, extract, isFromHostToCtr bool) error {
	srcPath, err := evalSymlinks(srcPath)
	if err != nil {
		return errors.Wrapf(err, "error evaluating symlinks %q", srcPath)
	}

	srcPath, srcfi, err := getPathInfo(srcPath)
	if err != nil {
		return err
	}

	filename := filepath.Base(destPath)
	if filename == "-" && !isFromHostToCtr {
		err := streamFileToStdout(srcPath, srcfi)
		if err != nil {
			return errors.Wrapf(err, "error streaming source file %s to Stdout", srcPath)
		}
		return nil
	}

	destdir := destPath
	if !srcfi.IsDir() {
		destdir = filepath.Dir(destPath)
	}
	_, err = os.Stat(destdir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	destDirIsExist := err == nil
	if err = os.MkdirAll(destdir, 0755); err != nil {
		return err
	}

	// return functions for copying items
	copyFileWithTar := chrootarchive.CopyFileWithTarAndChown(chownOpts, digest.Canonical.Digester().Hash(), idMappingOpts.UIDMap, idMappingOpts.GIDMap)
	copyWithTar := chrootarchive.CopyWithTarAndChown(chownOpts, digest.Canonical.Digester().Hash(), idMappingOpts.UIDMap, idMappingOpts.GIDMap)
	untarPath := chrootarchive.UntarPathAndChown(chownOpts, digest.Canonical.Digester().Hash(), idMappingOpts.UIDMap, idMappingOpts.GIDMap)

	if srcfi.IsDir() {
		logrus.Debugf("copying %q to %q", srcPath+string(os.PathSeparator)+"*", dest+string(os.PathSeparator)+"*")
		if destDirIsExist && !strings.HasSuffix(src, fmt.Sprintf("%s.", string(os.PathSeparator))) {
			srcPathBase := filepath.Base(srcPath)
			if !isFromHostToCtr {
				pathArr := strings.SplitN(src, ":", 2)
				if len(pathArr) != 2 {
					return errors.Errorf("invalid arguments %s, you must specify source path", src)
				}
				if pathArr[1] == "/" {
					// If `srcPath` is the root directory of the container,
					// `srcPath` will be `.../${sha256_ID}/merged/`, so do not join it
					srcPathBase = ""
				}
			}
			destPath = filepath.Join(destPath, srcPathBase)
		}
		if err = copyWithTar(srcPath, destPath); err != nil {
			return errors.Wrapf(err, "error copying %q to %q", srcPath, dest)
		}
		return nil
	}

	if extract {
		// We're extracting an archive into the destination directory.
		logrus.Debugf("extracting contents of %q into %q", srcPath, destPath)
		if err = untarPath(srcPath, destPath); err != nil {
			return errors.Wrapf(err, "error extracting %q into %q", srcPath, destPath)
		}
		return nil
	}

	destfi, err := os.Stat(destPath)
	if err != nil {
		if !os.IsNotExist(err) || strings.HasSuffix(dest, string(os.PathSeparator)) {
			return err
		}
	}
	if destfi != nil && destfi.IsDir() {
		destPath = filepath.Join(destPath, filepath.Base(srcPath))
	}

	// Copy the file, preserving attributes.
	logrus.Debugf("copying %q to %q", srcPath, destPath)
	if err = copyFileWithTar(srcPath, destPath); err != nil {
		return errors.Wrapf(err, "error copying %q to %q", srcPath, destPath)
	}
	return nil
}

func convertIDMap(idMaps []idtools.IDMap) (convertedIDMap []specs.LinuxIDMapping) {
	for _, idmap := range idMaps {
		tempIDMap := specs.LinuxIDMapping{
			ContainerID: uint32(idmap.ContainerID),
			HostID:      uint32(idmap.HostID),
			Size:        uint32(idmap.Size),
		}
		convertedIDMap = append(convertedIDMap, tempIDMap)
	}
	return convertedIDMap
}

func streamFileToStdout(srcPath string, srcfi os.FileInfo) error {
	if srcfi.IsDir() {
		tw := tar.NewWriter(os.Stdout)
		err := filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || !info.Mode().IsRegular() || path == srcPath {
				return err
			}
			hdr, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}

			if err = tw.WriteHeader(hdr); err != nil {
				return err
			}
			fh, err := os.Open(path)
			if err != nil {
				return err
			}
			defer fh.Close()

			_, err = io.Copy(tw, fh)
			return err
		})
		if err != nil {
			return errors.Wrapf(err, "error streaming directory %s to Stdout", srcPath)
		}
		return nil
	}

	file, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer file.Close()
	if !archive.IsArchivePath(srcPath) {
		tw := tar.NewWriter(os.Stdout)
		hdr, err := tar.FileInfoHeader(srcfi, "")
		if err != nil {
			return err
		}
		err = tw.WriteHeader(hdr)
		if err != nil {
			return err
		}
		_, err = io.Copy(tw, file)
		if err != nil {
			return errors.Wrapf(err, "error streaming archive %s to Stdout", srcPath)
		}
		return nil
	}

	_, err = io.Copy(os.Stdout, file)
	if err != nil {
		return errors.Wrapf(err, "error streaming file to Stdout")
	}
	return nil
}

func isVolumeDestName(path string, ctr *libpod.Container) (bool, string, string) {
	separator := string(os.PathSeparator)
	if filepath.IsAbs(path) {
		path = strings.TrimPrefix(path, separator)
	}
	if path == "" {
		return false, "", ""
	}
	for _, vol := range ctr.Config().NamedVolumes {
		volNamePath := strings.TrimPrefix(vol.Dest, separator)
		if matchVolumePath(path, volNamePath) {
			return true, vol.Dest, vol.Name
		}
	}
	return false, "", ""
}

// if SRCPATH or DESTPATH is from volume mount's destination -v or --mount type=volume, generates the path with volume mount point
func pathWithVolumeMount(runtime *libpod.Runtime, volDestName, volName, path string) (string, error) {
	destVolume, err := runtime.GetVolume(volName)
	if err != nil {
		return "", errors.Wrapf(err, "error getting volume destination %s", volName)
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(string(os.PathSeparator), path)
	}
	path, err = securejoin.SecureJoin(destVolume.MountPoint(), strings.TrimPrefix(path, volDestName))
	return path, err
}

func isBindMountDestName(path string, ctr *libpod.Container) (bool, specs.Mount) {
	separator := string(os.PathSeparator)
	if filepath.IsAbs(path) {
		path = strings.TrimPrefix(path, string(os.PathSeparator))
	}
	if path == "" {
		return false, specs.Mount{}
	}
	for _, m := range ctr.Config().Spec.Mounts {
		if m.Type != "bind" {
			continue
		}
		mDest := strings.TrimPrefix(m.Destination, separator)
		if matchVolumePath(path, mDest) {
			return true, m
		}
	}
	return false, specs.Mount{}
}

func matchVolumePath(path, target string) bool {
	pathStr := filepath.Clean(path)
	target = filepath.Clean(target)
	for len(pathStr) > len(target) && strings.Contains(pathStr, string(os.PathSeparator)) {
		pathStr = pathStr[:strings.LastIndex(pathStr, string(os.PathSeparator))]
	}
	return pathStr == target
}

func pathWithBindMountSource(m specs.Mount, path string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(string(os.PathSeparator), path)
	}
	return securejoin.SecureJoin(m.Source, strings.TrimPrefix(path, m.Destination))
}
