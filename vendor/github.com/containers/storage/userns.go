package storage

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	drivers "github.com/containers/storage/drivers"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/unshare"
	"github.com/containers/storage/types"
	libcontainerUser "github.com/opencontainers/runc/libcontainer/user"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// getAdditionalSubIDs looks up the additional IDs configured for
// the specified user.
// The argument USERNAME is ignored for rootless users, as it is not
// possible to use an arbitrary entry in /etc/sub*id.
// Differently, if the username is not specified for root users, a
// default name is used.
func getAdditionalSubIDs(username string) (*idSet, *idSet, error) {
	var uids, gids *idSet

	if unshare.IsRootless() {
		username = os.Getenv("USER")
		if username == "" {
			var id string
			if os.Geteuid() == 0 {
				id = strconv.Itoa(unshare.GetRootlessUID())
			} else {
				id = strconv.Itoa(os.Geteuid())
			}
			userID, err := user.LookupId(id)
			if err == nil {
				username = userID.Username
			}
		}
	} else if username == "" {
		username = RootAutoUserNsUser
	}
	mappings, err := idtools.NewIDMappings(username, username)
	if err != nil {
		logrus.Errorf("Cannot find mappings for user %q: %v", username, err)
	} else {
		uids = getHostIDs(mappings.UIDs())
		gids = getHostIDs(mappings.GIDs())
	}
	return uids, gids, nil
}

// getAvailableIDs returns the list of ranges that are usable by the current user.
// When running as root, it looks up the additional IDs assigned to the specified user.
// When running as rootless, the mappings assigned to the unprivileged user are converted
// to the IDs inside of the initial rootless user namespace.
func (s *store) getAvailableIDs() (*idSet, *idSet, error) {
	if s.additionalUIDs == nil {
		uids, gids, err := getAdditionalSubIDs(s.autoUsernsUser)
		if err != nil {
			return nil, nil, err
		}
		// Store the result so we don't need to look it up again next time
		s.additionalUIDs, s.additionalGIDs = uids, gids
	}

	if !unshare.IsRootless() {
		// No mapping to inner namespace needed
		return s.additionalUIDs, s.additionalGIDs, nil
	}

	// We are already inside of the rootless user namespace.
	// We need to remap the configured mappings to what is available
	// inside of the rootless userns.
	u := newIDSet([]interval{{start: 1, end: s.additionalUIDs.size() + 1}})
	g := newIDSet([]interval{{start: 1, end: s.additionalGIDs.size() + 1}})
	return u, g, nil
}

// parseMountedFiles returns the maximum UID and GID found in the /etc/passwd and
// /etc/group files.
func parseMountedFiles(containerMount, passwdFile, groupFile string) uint32 {
	if passwdFile == "" {
		passwdFile = filepath.Join(containerMount, "etc/passwd")
	}
	if groupFile == "" {
		groupFile = filepath.Join(groupFile, "etc/group")
	}

	size := 0

	users, err := libcontainerUser.ParsePasswdFile(passwdFile)
	if err == nil {
		for _, u := range users {
			// Skip the "nobody" user otherwise we end up with 65536
			// ids with most images
			if u.Name == "nobody" {
				continue
			}
			if u.Uid > size {
				size = u.Uid
			}
			if u.Gid > size {
				size = u.Gid
			}
		}
	}

	groups, err := libcontainerUser.ParseGroupFile(groupFile)
	if err == nil {
		for _, g := range groups {
			if g.Name == "nobody" {
				continue
			}
			if g.Gid > size {
				size = g.Gid
			}
		}
	}

	return uint32(size)
}

// getMaxSizeFromImage returns the maximum ID used by the specified image.
// The layer stores must be already locked.
func (s *store) getMaxSizeFromImage(id string, image *Image, passwdFile, groupFile string) (uint32, error) {
	lstore, err := s.LayerStore()
	if err != nil {
		return 0, err
	}
	lstores, err := s.ROLayerStores()
	if err != nil {
		return 0, err
	}

	size := uint32(0)

	var topLayer *Layer
	layerName := image.TopLayer
outer:
	for {
		for _, ls := range append([]ROLayerStore{lstore}, lstores...) {
			layer, err := ls.Get(layerName)
			if err != nil {
				continue
			}
			if image.TopLayer == layerName {
				topLayer = layer
			}
			for _, uid := range layer.UIDs {
				if uid >= size {
					size = uid + 1
				}
			}
			for _, gid := range layer.GIDs {
				if gid >= size {
					size = gid + 1
				}
			}
			layerName = layer.Parent
			if layerName == "" {
				break outer
			}
			continue outer
		}
		return 0, errors.Errorf("cannot find layer %q", layerName)
	}

	rlstore, err := s.LayerStore()
	if err != nil {
		return 0, err
	}

	layerOptions := &LayerOptions{
		IDMappingOptions: types.IDMappingOptions{
			HostUIDMapping: true,
			HostGIDMapping: true,
			UIDMap:         nil,
			GIDMap:         nil,
		},
	}

	// We need to create a temporary layer so we can mount it and lookup the
	// maximum IDs used.
	clayer, err := rlstore.Create(id, topLayer, nil, "", nil, layerOptions, false)
	if err != nil {
		return 0, err
	}
	defer rlstore.Delete(clayer.ID)

	mountOptions := drivers.MountOpts{
		MountLabel: "",
		UidMaps:    nil,
		GidMaps:    nil,
		Options:    nil,
	}

	mountpoint, err := rlstore.Mount(clayer.ID, mountOptions)
	if err != nil {
		return 0, err
	}
	defer rlstore.Unmount(clayer.ID, true)

	userFilesSize := parseMountedFiles(mountpoint, passwdFile, groupFile)
	if userFilesSize > size {
		size = userFilesSize
	}

	return size, nil
}

// getAutoUserNS creates an automatic user namespace
func (s *store) getAutoUserNS(id string, options *types.AutoUserNsOptions, image *Image) ([]idtools.IDMap, []idtools.IDMap, error) {
	requestedSize := uint32(0)
	initialSize := uint32(1)
	if options.Size > 0 {
		requestedSize = options.Size
	}
	if options.InitialSize > 0 {
		initialSize = options.InitialSize
	}

	availableUIDs, availableGIDs, err := s.getAvailableIDs()
	if err != nil {
		return nil, nil, errors.Wrapf(err, "cannot read mappings")
	}

	// Look every container that is using a user namespace and store
	// the intervals that are already used.
	containers, err := s.Containers()
	if err != nil {
		return nil, nil, err
	}
	var usedUIDs, usedGIDs []idtools.IDMap
	for _, c := range containers {
		usedUIDs = append(usedUIDs, c.UIDMap...)
		usedGIDs = append(usedGIDs, c.GIDMap...)
	}

	size := requestedSize

	// If there is no requestedSize, lookup the maximum used IDs in the layers
	// metadata.  Make sure the size is at least s.autoNsMinSize and it is not
	// bigger than s.autoNsMaxSize.
	// This is a best effort heuristic.
	if requestedSize == 0 {
		size = initialSize
		if s.autoNsMinSize > size {
			size = s.autoNsMinSize
		}
		if image != nil {
			sizeFromImage, err := s.getMaxSizeFromImage(id, image, options.PasswdFile, options.GroupFile)
			if err != nil {
				return nil, nil, err
			}
			if sizeFromImage > size {
				size = sizeFromImage
			}
		}
		if s.autoNsMaxSize > 0 && size > s.autoNsMaxSize {
			return nil, nil, errors.Errorf("the container needs a user namespace with size %q that is bigger than the maximum value allowed with userns=auto %q", size, s.autoNsMaxSize)
		}
	}

	return getAutoUserNSIDMappings(
		int(size),
		availableUIDs, availableGIDs,
		usedUIDs, usedGIDs,
		options.AdditionalUIDMappings, options.AdditionalGIDMappings,
	)
}

// getAutoUserNSIDMappings computes the user/group id mappings for the automatic user namespace.
func getAutoUserNSIDMappings(
	size int,
	availableUIDs, availableGIDs *idSet,
	usedUIDMappings, usedGIDMappings, additionalUIDMappings, additionalGIDMappings []idtools.IDMap,
) ([]idtools.IDMap, []idtools.IDMap, error) {
	usedUIDs := getHostIDs(append(usedUIDMappings, additionalUIDMappings...))
	usedGIDs := getHostIDs(append(usedGIDMappings, additionalGIDMappings...))

	// Exclude additional uids and gids from requested range.
	targetIDs := newIDSet([]interval{{start: 0, end: size}})
	requestedContainerUIDs := targetIDs.subtract(getContainerIDs(additionalUIDMappings))
	requestedContainerGIDs := targetIDs.subtract(getContainerIDs(additionalGIDMappings))

	// Make sure the specified additional IDs are not used as part of the automatic
	// mapping
	availableUIDs, err := availableUIDs.subtract(usedUIDs).findAvailable(requestedContainerUIDs.size())
	if err != nil {
		return nil, nil, err
	}
	availableGIDs, err = availableGIDs.subtract(usedGIDs).findAvailable(requestedContainerGIDs.size())
	if err != nil {
		return nil, nil, err
	}

	uidMap := append(availableUIDs.zip(requestedContainerUIDs), additionalUIDMappings...)
	gidMap := append(availableGIDs.zip(requestedContainerGIDs), additionalGIDMappings...)
	return uidMap, gidMap, nil
}
