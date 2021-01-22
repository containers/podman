package abi

import (
	"context"
	"io"
	"strings"

	buildahCopiah "github.com/containers/buildah/copier"
	"github.com/containers/buildah/pkg/chrootuser"
	"github.com/containers/buildah/util"
	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/idtools"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// NOTE: Only the parent directory of the container path must exist.  The path
// itself may be created while copying.
func (ic *ContainerEngine) ContainerCopyFromArchive(ctx context.Context, nameOrID string, containerPath string, reader io.Reader) (entities.ContainerCopyFunc, error) {
	container, err := ic.Libpod.LookupContainer(nameOrID)
	if err != nil {
		return nil, err
	}

	containerMountPoint, err := container.Mount()
	if err != nil {
		return nil, err
	}

	unmount := func() {
		if err := container.Unmount(false); err != nil {
			logrus.Errorf("Error unmounting container: %v", err)
		}
	}

	_, resolvedRoot, resolvedContainerPath, err := ic.containerStat(container, containerMountPoint, containerPath)
	if err != nil {
		unmount()
		return nil, err
	}

	decompressed, err := archive.DecompressStream(reader)
	if err != nil {
		unmount()
		return nil, err
	}

	idMappings, idPair, err := getIDMappingsAndPair(container, resolvedRoot)
	if err != nil {
		unmount()
		return nil, err
	}

	logrus.Debugf("Container copy *to* %q (resolved: %q) on container %q (ID: %s)", containerPath, resolvedContainerPath, container.Name(), container.ID())

	return func() error {
		defer unmount()
		defer decompressed.Close()
		putOptions := buildahCopiah.PutOptions{
			UIDMap:     idMappings.UIDMap,
			GIDMap:     idMappings.GIDMap,
			ChownDirs:  idPair,
			ChownFiles: idPair,
		}
		return buildahCopiah.Put(resolvedRoot, resolvedContainerPath, putOptions, decompressed)
	}, nil
}

func (ic *ContainerEngine) ContainerCopyToArchive(ctx context.Context, nameOrID string, containerPath string, writer io.Writer) (entities.ContainerCopyFunc, error) {
	container, err := ic.Libpod.LookupContainer(nameOrID)
	if err != nil {
		return nil, err
	}

	containerMountPoint, err := container.Mount()
	if err != nil {
		return nil, err
	}

	unmount := func() {
		if err := container.Unmount(false); err != nil {
			logrus.Errorf("Error unmounting container: %v", err)
		}
	}

	// Make sure that "/" copies the *contents* of the mount point and not
	// the directory.
	if containerPath == "/" {
		containerPath = "/."
	}

	_, resolvedRoot, resolvedContainerPath, err := ic.containerStat(container, containerMountPoint, containerPath)
	if err != nil {
		unmount()
		return nil, err
	}

	idMappings, idPair, err := getIDMappingsAndPair(container, resolvedRoot)
	if err != nil {
		unmount()
		return nil, err
	}

	logrus.Debugf("Container copy *from* %q (resolved: %q) on container %q (ID: %s)", containerPath, resolvedContainerPath, container.Name(), container.ID())

	return func() error {
		defer container.Unmount(false)
		getOptions := buildahCopiah.GetOptions{
			// Unless the specified path ends with ".", we want to copy the base directory.
			KeepDirectoryNames: !strings.HasSuffix(resolvedContainerPath, "."),
			UIDMap:             idMappings.UIDMap,
			GIDMap:             idMappings.GIDMap,
			ChownDirs:          idPair,
			ChownFiles:         idPair,
		}
		return buildahCopiah.Get(resolvedRoot, "", getOptions, []string{resolvedContainerPath}, writer)
	}, nil
}

// getIDMappingsAndPair returns the ID mappings for the container and the host
// ID pair.
func getIDMappingsAndPair(container *libpod.Container, containerMount string) (*storage.IDMappingOptions, *idtools.IDPair, error) {
	user, err := getContainerUser(container, containerMount)
	if err != nil {
		return nil, nil, err
	}

	idMappingOpts, err := container.IDMappings()
	if err != nil {
		return nil, nil, err
	}

	hostUID, hostGID, err := util.GetHostIDs(idtoolsToRuntimeSpec(idMappingOpts.UIDMap), idtoolsToRuntimeSpec(idMappingOpts.GIDMap), user.UID, user.GID)
	if err != nil {
		return nil, nil, err
	}

	idPair := idtools.IDPair{UID: int(hostUID), GID: int(hostGID)}
	return &idMappingOpts, &idPair, nil
}

// getContainerUser returns the specs.User of the container.
func getContainerUser(container *libpod.Container, mountPoint string) (specs.User, error) {
	userspec := container.Config().User

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

// idtoolsToRuntimeSpec converts idtools ID mapping to the one of the runtime spec.
func idtoolsToRuntimeSpec(idMaps []idtools.IDMap) (convertedIDMap []specs.LinuxIDMapping) {
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
