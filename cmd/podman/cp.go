package main

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/buildah/pkg/chrootuser"
	"github.com/containers/buildah/util"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/cgroups"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/chrootarchive"
	"github.com/containers/storage/pkg/idtools"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	cpCommand cliconfig.CpValues

	cpDescription = `Command copies the contents of SRC_PATH to the DEST_PATH.

  You can copy from the container's file system to the local machine or the reverse, from the local filesystem to the container. If "-" is specified for either the SRC_PATH or DEST_PATH, you can also stream a tar archive from STDIN or to STDOUT. The CONTAINER can be a running or stopped container.  The SRC_PATH or DEST_PATH can be a file or directory.
`
	_cpCommand = &cobra.Command{
		Use:   "cp [flags] SRC_PATH DEST_PATH",
		Short: "Copy files/folders between a container and the local filesystem",
		Long:  cpDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			cpCommand.InputArgs = args
			cpCommand.GlobalFlags = MainGlobalOpts
			cpCommand.Remote = remoteclient
			return cpCmd(&cpCommand)
		},
		Example: "[CONTAINER:]SRC_PATH [CONTAINER:]DEST_PATH",
	}
)

func init() {
	cpCommand.Command = _cpCommand
	flags := cpCommand.Flags()
	flags.BoolVar(&cpCommand.Extract, "extract", false, "Extract the tar file into the destination directory.")
	flags.BoolVar(&cpCommand.Pause, "pause", copyPause(), "Pause the container while copying")
	cpCommand.SetHelpTemplate(HelpTemplate())
	cpCommand.SetUsageTemplate(UsageTemplate())
}

func cpCmd(c *cliconfig.CpValues) error {
	args := c.InputArgs
	if len(args) != 2 {
		return errors.Errorf("you must provide a source path and a destination path")
	}

	runtime, err := libpodruntime.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.DeferredShutdown(false)

	return copyBetweenHostAndContainer(runtime, args[0], args[1], c.Extract, c.Pause)
}

func copyBetweenHostAndContainer(runtime *libpod.Runtime, src string, dest string, extract bool, pause bool) error {

	srcCtr, srcPath := parsePath(runtime, src)
	destCtr, destPath := parsePath(runtime, dest)

	if (srcCtr == nil && destCtr == nil) || (srcCtr != nil && destCtr != nil) {
		return errors.Errorf("invalid arguments %s, %s you must use just one container", src, dest)
	}

	if len(srcPath) == 0 || len(destPath) == 0 {
		return errors.Errorf("invalid arguments %s, %s you must specify paths", src, dest)
	}
	ctr := srcCtr
	isFromHostToCtr := ctr == nil
	if isFromHostToCtr {
		ctr = destCtr
	}

	mountPoint, err := ctr.Mount()
	if err != nil {
		return err
	}
	defer func() {
		if err := ctr.Unmount(false); err != nil {
			logrus.Errorf("unable to umount container '%s': %q", ctr.ID(), err)
		}
	}()

	// We can't pause rootless containers.
	if pause && rootless.IsRootless() {
		state, err := ctr.State()
		if err != nil {
			return err
		}
		if state == define.ContainerStateRunning {
			return errors.Errorf("cannot copy into running rootless container with pause set - pass --pause=false to force copying")
		}
	}

	if pause && !rootless.IsRootless() {
		if err := ctr.Pause(); err != nil {
			// An invalid state error is fine.
			// The container isn't running or is already paused.
			// TODO: We can potentially start the container while
			// the copy is running, which still allows a race where
			// malicious code could mess with the symlink.
			if errors.Cause(err) != define.ErrCtrStateInvalid {
				return err
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
		return err
	}
	idMappingOpts, err := ctr.IDMappings()
	if err != nil {
		return errors.Wrapf(err, "error getting IDMappingOptions")
	}
	destOwner := idtools.IDPair{UID: int(user.UID), GID: int(user.GID)}
	hostUID, hostGID, err := util.GetHostIDs(convertIDMap(idMappingOpts.UIDMap), convertIDMap(idMappingOpts.GIDMap), user.UID, user.GID)
	if err != nil {
		return err
	}

	hostOwner := idtools.IDPair{UID: int(hostUID), GID: int(hostGID)}

	if isFromHostToCtr {
		if isVol, volDestName, volName := isVolumeDestName(destPath, ctr); isVol {
			path, err := pathWithVolumeMount(ctr, runtime, volDestName, volName, destPath)
			if err != nil {
				return errors.Wrapf(err, "error getting destination path from volume %s", volDestName)
			}
			destPath = path
		} else if isBindMount, mount := isBindMountDestName(destPath, ctr); isBindMount {
			path, err := pathWithBindMountSource(mount, destPath)
			if err != nil {
				return errors.Wrapf(err, "error getting destination path from bind mount %s", mount.Destination)
			}
			destPath = path
		} else if filepath.IsAbs(destPath) {
			cleanedPath, err := securejoin.SecureJoin(mountPoint, destPath)
			if err != nil {
				return err
			}
			destPath = cleanedPath
		} else {
			ctrWorkDir, err := securejoin.SecureJoin(mountPoint, ctr.WorkingDir())
			if err != nil {
				return err
			}
			if err = idtools.MkdirAllAndChownNew(ctrWorkDir, 0755, hostOwner); err != nil {
				return errors.Wrapf(err, "error creating directory %q", destPath)
			}
			cleanedPath, err := securejoin.SecureJoin(mountPoint, filepath.Join(ctr.WorkingDir(), destPath))
			if err != nil {
				return err
			}
			destPath = cleanedPath
		}
	} else {
		destOwner = idtools.IDPair{UID: os.Getuid(), GID: os.Getgid()}
		if isVol, volDestName, volName := isVolumeDestName(srcPath, ctr); isVol {
			path, err := pathWithVolumeMount(ctr, runtime, volDestName, volName, srcPath)
			if err != nil {
				return errors.Wrapf(err, "error getting source path from volume %s", volDestName)
			}
			srcPath = path
		} else if isBindMount, mount := isBindMountDestName(srcPath, ctr); isBindMount {
			path, err := pathWithBindMountSource(mount, srcPath)
			if err != nil {
				return errors.Wrapf(err, "error getting source path from bind moutn %s", mount.Destination)
			}
			srcPath = path
		} else if filepath.IsAbs(srcPath) {
			cleanedPath, err := securejoin.SecureJoin(mountPoint, srcPath)
			if err != nil {
				return err
			}
			srcPath = cleanedPath
		} else {
			cleanedPath, err := securejoin.SecureJoin(mountPoint, filepath.Join(ctr.WorkingDir(), srcPath))
			if err != nil {
				return err
			}
			srcPath = cleanedPath
		}
	}

	if !filepath.IsAbs(destPath) {
		dir, err := os.Getwd()
		if err != nil {
			return errors.Wrapf(err, "err getting current working directory")
		}
		destPath = filepath.Join(dir, destPath)
	}

	if src == "-" {
		srcPath = os.Stdin.Name()
		extract = true
	}
	return copy(srcPath, destPath, dest, idMappingOpts, &destOwner, extract, isFromHostToCtr)
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

func getPathInfo(path string) (string, os.FileInfo, error) {
	path, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", nil, errors.Wrapf(err, "error evaluating symlinks %q", path)
	}
	srcfi, err := os.Stat(path)
	if err != nil {
		return "", nil, errors.Wrapf(err, "error reading path %q", path)
	}
	return path, srcfi, nil
}

func copy(src, destPath, dest string, idMappingOpts storage.IDMappingOptions, chownOpts *idtools.IDPair, extract, isFromHostToCtr bool) error {
	srcPath, err := filepath.EvalSymlinks(src)
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
		return errors.Wrapf(err, "error checking directory %q", destdir)
	}
	destDirIsExist := err == nil
	if err = os.MkdirAll(destdir, 0755); err != nil {
		return errors.Wrapf(err, "error creating directory %q", destdir)
	}

	// return functions for copying items
	copyFileWithTar := chrootarchive.CopyFileWithTarAndChown(chownOpts, digest.Canonical.Digester().Hash(), idMappingOpts.UIDMap, idMappingOpts.GIDMap)
	copyWithTar := chrootarchive.CopyWithTarAndChown(chownOpts, digest.Canonical.Digester().Hash(), idMappingOpts.UIDMap, idMappingOpts.GIDMap)
	untarPath := chrootarchive.UntarPathAndChown(chownOpts, digest.Canonical.Digester().Hash(), idMappingOpts.UIDMap, idMappingOpts.GIDMap)

	if srcfi.IsDir() {
		logrus.Debugf("copying %q to %q", srcPath+string(os.PathSeparator)+"*", dest+string(os.PathSeparator)+"*")
		if destDirIsExist && !strings.HasSuffix(src, fmt.Sprintf("%s.", string(os.PathSeparator))) {
			destPath = filepath.Join(destPath, filepath.Base(srcPath))
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
			return errors.Wrapf(err, "failed to get stat of dest path %s", destPath)
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
		return errors.Wrapf(err, "error opening file %s", srcPath)
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
func pathWithVolumeMount(ctr *libpod.Container, runtime *libpod.Runtime, volDestName, volName, path string) (string, error) {
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

func copyPause() bool {
	if !remoteclient && rootless.IsRootless() {
		cgroupv2, _ := cgroups.IsCgroup2UnifiedMode()
		if !cgroupv2 {
			logrus.Debugf("defaulting to pause==false on rootless cp in cgroupv1 systems")
			return false
		}
	}
	return true
}
