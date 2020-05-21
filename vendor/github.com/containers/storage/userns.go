package storage

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	drivers "github.com/containers/storage/drivers"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/unshare"
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
func getAdditionalSubIDs(username string) ([]idtools.IDMap, []idtools.IDMap, error) {
	var uids, gids []idtools.IDMap

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
		logrus.Errorf("cannot find mappings for user %q: %v", username, err)
	} else {
		uids = mappings.UIDs()
		gids = mappings.GIDs()
	}
	return uids, gids, nil
}

// getAvailableMappings returns the list of ranges that are usable by the current user.
// When running as root, it looks up the additional IDs assigned to the specified user.
// When running as rootless, the mappings assigned to the unprivileged user are converted
// to the IDs inside of the initial rootless user namespace.
func (s *store) getAvailableMappings() ([]idtools.IDMap, []idtools.IDMap, error) {
	if s.autoUIDMap == nil {
		uids, gids, err := getAdditionalSubIDs(s.autoUsernsUser)
		if err != nil {
			return nil, nil, err
		}
		// Store the result so we don't need to look it up again next time
		s.autoUIDMap, s.autoGIDMap = uids, gids
	}

	uids := s.autoUIDMap
	gids := s.autoGIDMap

	if !unshare.IsRootless() {
		// No mapping to inner namespace needed
		return copyIDMap(uids), copyIDMap(gids), nil
	}

	// We are already inside of the rootless user namespace.
	// We need to remap the configured mappings to what is available
	// inside of the rootless userns.
	totaluid := 0
	totalgid := 0
	for _, u := range uids {
		totaluid += u.Size
	}
	for _, g := range gids {
		totalgid += g.Size
	}

	u := []idtools.IDMap{{ContainerID: 0, HostID: 1, Size: totaluid}}
	g := []idtools.IDMap{{ContainerID: 0, HostID: 1, Size: totalgid}}
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
		IDMappingOptions: IDMappingOptions{
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

// subtractHostIDs return the subtraction of the range USED from AVAIL.  The range is specified
// by [HostID, HostID+Size).
// ContainerID is ignored.
func subtractHostIDs(avail idtools.IDMap, used idtools.IDMap) []idtools.IDMap {
	switch {
	case used.HostID <= avail.HostID && used.HostID+used.Size >= avail.HostID+avail.Size:
		return nil
	case used.HostID <= avail.HostID && used.HostID+used.Size > avail.HostID && used.HostID+used.Size < avail.HostID+avail.Size:
		newContainerID := used.HostID + used.Size
		newHostID := used.HostID + used.Size
		r := idtools.IDMap{
			ContainerID: newContainerID,
			HostID:      newHostID,
			Size:        avail.Size + avail.HostID - newHostID,
		}
		return []idtools.IDMap{r}
	case used.HostID > avail.HostID && used.HostID < avail.HostID+avail.Size && used.HostID+used.Size >= avail.HostID+avail.Size:
		r := idtools.IDMap{
			ContainerID: avail.ContainerID,
			HostID:      avail.HostID,
			Size:        used.HostID - avail.HostID,
		}
		return []idtools.IDMap{r}
	case used.HostID > avail.HostID && used.HostID < avail.HostID+avail.Size && used.HostID+used.Size < avail.HostID+avail.Size:
		r1 := idtools.IDMap{
			ContainerID: avail.ContainerID,
			HostID:      avail.HostID,
			Size:        used.HostID - avail.HostID,
		}
		r2 := idtools.IDMap{
			ContainerID: used.ContainerID + used.Size,
			HostID:      used.HostID + used.Size,
			Size:        avail.HostID + avail.Size - used.HostID - used.Size,
		}
		return []idtools.IDMap{r1, r2}
	default:
		r := idtools.IDMap{
			ContainerID: 0,
			HostID:      avail.HostID,
			Size:        avail.Size,
		}
		return []idtools.IDMap{r}
	}
}

// subtractContainerIDs return the subtraction of the range USED from AVAIL.  The range is specified
// by [ContainerID, ContainerID+Size).
// HostID is ignored.
func subtractContainerIDs(avail idtools.IDMap, used idtools.IDMap) []idtools.IDMap {
	switch {
	case used.ContainerID <= avail.ContainerID && used.ContainerID+used.Size >= avail.ContainerID+avail.Size:
		return nil
	case used.ContainerID <= avail.ContainerID && used.ContainerID+used.Size > avail.ContainerID && used.ContainerID+used.Size < avail.ContainerID+avail.Size:
		newContainerID := used.ContainerID + used.Size
		newHostID := used.HostID + used.Size
		r := idtools.IDMap{
			ContainerID: newContainerID,
			HostID:      newHostID,
			Size:        avail.Size + avail.ContainerID - newContainerID,
		}
		return []idtools.IDMap{r}
	case used.ContainerID > avail.ContainerID && used.ContainerID < avail.ContainerID+avail.Size && used.ContainerID+used.Size >= avail.ContainerID+avail.Size:
		r := idtools.IDMap{
			ContainerID: avail.ContainerID,
			HostID:      avail.HostID,
			Size:        used.ContainerID - avail.ContainerID,
		}
		return []idtools.IDMap{r}
	case used.ContainerID > avail.ContainerID && used.ContainerID < avail.ContainerID+avail.Size && used.ContainerID+used.Size < avail.ContainerID+avail.Size:
		r1 := idtools.IDMap{
			ContainerID: avail.ContainerID,
			HostID:      avail.HostID,
			Size:        used.ContainerID - avail.ContainerID,
		}
		r2 := idtools.IDMap{
			ContainerID: used.ContainerID + used.Size,
			HostID:      used.HostID + used.Size,
			Size:        avail.ContainerID + avail.Size - used.ContainerID - used.Size,
		}
		return []idtools.IDMap{r1, r2}
	default:
		r := idtools.IDMap{
			ContainerID: avail.ContainerID,
			HostID:      avail.HostID,
			Size:        avail.Size,
		}
		return []idtools.IDMap{r}
	}
}

// subtractAll subtracts all usedIDs from the available IDs.
func subtractAll(availableIDs, usedIDs []idtools.IDMap, host bool) []idtools.IDMap {
	for _, u := range usedIDs {
		for i := 0; i < len(availableIDs); {
			var prev []idtools.IDMap
			if i > 0 {
				prev = availableIDs[:i-1]
			}
			next := availableIDs[i+1:]
			cur := availableIDs[i]
			var newRanges []idtools.IDMap
			if host {
				newRanges = subtractHostIDs(cur, u)
			} else {
				newRanges = subtractContainerIDs(cur, u)
			}
			availableIDs = append(append(prev, newRanges...), next...)
			i += len(newRanges)
		}
	}
	return availableIDs
}

// findAvailableIDRange returns the list of IDs that are not used by existing containers.
// This function is used to lookup both UIDs and GIDs.
func findAvailableIDRange(size uint32, availableIDs, usedIDs []idtools.IDMap) ([]idtools.IDMap, error) {
	var avail []idtools.IDMap

	// ContainerID will be adjusted later.
	for _, i := range availableIDs {
		n := idtools.IDMap{
			ContainerID: 0,
			HostID:      i.HostID,
			Size:        i.Size,
		}
		avail = append(avail, n)
	}
	avail = subtractAll(avail, usedIDs, true)

	currentID := 0
	remaining := size
	// We know the size for each intervals, let's adjust the ContainerID for each
	// of them.
	for i := 0; i < len(avail); i++ {
		avail[i].ContainerID = currentID
		if uint32(avail[i].Size) >= remaining {
			avail[i].Size = int(remaining)
			return avail[:i+1], nil
		}
		remaining -= uint32(avail[i].Size)
	}

	return nil, errors.New("could not find enough available IDs")
}

// findAvailableRange returns both the list of UIDs and GIDs ranges that are not
// currently used by other containers.
// It is a wrapper for findAvailableIDRange.
func findAvailableRange(sizeUID, sizeGID uint32, availableUIDs, availableGIDs, usedUIDs, usedGIDs []idtools.IDMap) ([]idtools.IDMap, []idtools.IDMap, error) {
	UIDMap, err := findAvailableIDRange(sizeUID, availableUIDs, usedUIDs)
	if err != nil {
		return nil, nil, err
	}

	GIDMap, err := findAvailableIDRange(sizeGID, availableGIDs, usedGIDs)
	if err != nil {
		return nil, nil, err
	}

	return UIDMap, GIDMap, nil
}

// getAutoUserNS creates an automatic user namespace
func (s *store) getAutoUserNS(id string, options *AutoUserNsOptions, image *Image) ([]idtools.IDMap, []idtools.IDMap, error) {
	requestedSize := uint32(0)
	initialSize := uint32(1)
	if options.Size > 0 {
		requestedSize = options.Size
	}
	if options.InitialSize > 0 {
		initialSize = options.InitialSize
	}

	availableUIDs, availableGIDs, err := s.getAvailableMappings()
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
	// Make sure the specified additional IDs are not used as part of the automatic
	// mapping
	usedUIDs = append(usedUIDs, options.AdditionalUIDMappings...)
	usedGIDs = append(usedGIDs, options.AdditionalGIDMappings...)
	availableUIDs, availableGIDs, err = findAvailableRange(size, size, availableUIDs, availableGIDs, usedUIDs, usedGIDs)
	if err != nil {
		return nil, nil, err
	}

	// We need to make sure the specified container IDs are also dropped from the automatic
	// namespaces we have found.
	if len(options.AdditionalUIDMappings) > 0 {
		availableUIDs = subtractAll(availableUIDs, options.AdditionalUIDMappings, false)
	}
	if len(options.AdditionalGIDMappings) > 0 {
		availableGIDs = subtractAll(availableGIDs, options.AdditionalGIDMappings, false)
	}

	return append(availableUIDs, options.AdditionalUIDMappings...), append(availableGIDs, options.AdditionalGIDMappings...), nil
}
