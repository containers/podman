//go:build linux
// +build linux

package libpod

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	metadata "github.com/checkpoint-restore/checkpointctl/lib"
	cdi "github.com/container-orchestrated-devices/container-device-interface/pkg"
	cnitypes "github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containers/buildah/pkg/chrootuser"
	"github.com/containers/buildah/pkg/overlay"
	butil "github.com/containers/buildah/util"
	"github.com/containers/common/pkg/apparmor"
	"github.com/containers/common/pkg/chown"
	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/subscriptions"
	"github.com/containers/common/pkg/umask"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/libpod/events"
	"github.com/containers/podman/v3/pkg/annotations"
	"github.com/containers/podman/v3/pkg/cgroups"
	"github.com/containers/podman/v3/pkg/checkpoint/crutils"
	"github.com/containers/podman/v3/pkg/criu"
	"github.com/containers/podman/v3/pkg/lookup"
	"github.com/containers/podman/v3/pkg/resolvconf"
	"github.com/containers/podman/v3/pkg/rootless"
	"github.com/containers/podman/v3/pkg/util"
	"github.com/containers/podman/v3/utils"
	"github.com/containers/podman/v3/version"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/idtools"
	securejoin "github.com/cyphar/filepath-securejoin"
	runcuser "github.com/opencontainers/runc/libcontainer/user"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func (c *Container) mountSHM(shmOptions string) error {
	if err := unix.Mount("shm", c.config.ShmDir, "tmpfs", unix.MS_NOEXEC|unix.MS_NOSUID|unix.MS_NODEV,
		label.FormatMountLabel(shmOptions, c.config.MountLabel)); err != nil {
		return errors.Wrapf(err, "failed to mount shm tmpfs %q", c.config.ShmDir)
	}
	return nil
}

func (c *Container) unmountSHM(mount string) error {
	if err := unix.Unmount(mount, 0); err != nil {
		if err != syscall.EINVAL && err != syscall.ENOENT {
			return errors.Wrapf(err, "error unmounting container %s SHM mount %s", c.ID(), mount)
		}
		// If it's just an EINVAL or ENOENT, debug logs only
		logrus.Debugf("container %s failed to unmount %s : %v", c.ID(), mount, err)
	}
	return nil
}

// prepare mounts the container and sets up other required resources like net
// namespaces
func (c *Container) prepare() error {
	var (
		wg                              sync.WaitGroup
		netNS                           ns.NetNS
		networkStatus                   []*cnitypes.Result
		createNetNSErr, mountStorageErr error
		mountPoint                      string
		tmpStateLock                    sync.Mutex
	)

	wg.Add(2)

	go func() {
		defer wg.Done()
		// Set up network namespace if not already set up
		noNetNS := c.state.NetNS == nil
		if c.config.CreateNetNS && noNetNS && !c.config.PostConfigureNetNS {
			netNS, networkStatus, createNetNSErr = c.runtime.createNetNS(c)
			if createNetNSErr != nil {
				return
			}

			tmpStateLock.Lock()
			defer tmpStateLock.Unlock()

			// Assign NetNS attributes to container
			c.state.NetNS = netNS
			c.state.NetworkStatus = networkStatus
		}

		// handle rootless network namespace setup
		if noNetNS && !c.config.PostConfigureNetNS {
			if rootless.IsRootless() {
				createNetNSErr = c.runtime.setupRootlessNetNS(c)
			} else if c.config.NetMode.IsSlirp4netns() {
				createNetNSErr = c.runtime.setupSlirp4netns(c)
			}
		}
	}()
	// Mount storage if not mounted
	go func() {
		defer wg.Done()
		mountPoint, mountStorageErr = c.mountStorage()

		if mountStorageErr != nil {
			return
		}

		tmpStateLock.Lock()
		defer tmpStateLock.Unlock()

		// Finish up mountStorage
		c.state.Mounted = true
		c.state.Mountpoint = mountPoint

		logrus.Debugf("Created root filesystem for container %s at %s", c.ID(), c.state.Mountpoint)
	}()

	wg.Wait()

	var createErr error
	if createNetNSErr != nil {
		createErr = createNetNSErr
	}
	if mountStorageErr != nil {
		if createErr != nil {
			logrus.Errorf("Error preparing container %s: %v", c.ID(), createErr)
		}
		createErr = mountStorageErr
	}

	// Only trigger storage cleanup if mountStorage was successful.
	// Otherwise, we may mess up mount counters.
	if createNetNSErr != nil && mountStorageErr == nil {
		if err := c.cleanupStorage(); err != nil {
			// createErr is guaranteed non-nil, so print
			// unconditionally
			logrus.Errorf("Error preparing container %s: %v", c.ID(), createErr)
			createErr = errors.Wrapf(err, "error unmounting storage for container %s after network create failure", c.ID())
		}
	}

	// It's OK to unconditionally trigger network cleanup. If the network
	// isn't ready it will do nothing.
	if createErr != nil {
		if err := c.cleanupNetwork(); err != nil {
			logrus.Errorf("Error preparing container %s: %v", c.ID(), createErr)
			createErr = errors.Wrapf(err, "error cleaning up container %s network after setup failure", c.ID())
		}
	}

	if createErr != nil {
		return createErr
	}

	// Save changes to container state
	if err := c.save(); err != nil {
		return err
	}

	return nil
}

// resolveWorkDir resolves the container's workdir and, depending on the
// configuration, will create it, or error out if it does not exist.
// Note that the container must be mounted before.
func (c *Container) resolveWorkDir() error {
	workdir := c.WorkingDir()

	// If the specified workdir is a subdir of a volume or mount,
	// we don't need to do anything.  The runtime is taking care of
	// that.
	if isPathOnVolume(c, workdir) || isPathOnBindMount(c, workdir) {
		logrus.Debugf("Workdir %q resolved to a volume or mount", workdir)
		return nil
	}

	_, resolvedWorkdir, err := c.resolvePath(c.state.Mountpoint, workdir)
	if err != nil {
		return err
	}
	logrus.Debugf("Workdir %q resolved to host path %q", workdir, resolvedWorkdir)

	st, err := os.Stat(resolvedWorkdir)
	if err == nil {
		if !st.IsDir() {
			return errors.Errorf("workdir %q exists on container %s, but is not a directory", workdir, c.ID())
		}
		return nil
	}
	if !c.config.CreateWorkingDir {
		// No need to create it (e.g., `--workdir=/foo`), so let's make sure
		// the path exists on the container.
		if err != nil {
			if os.IsNotExist(err) {
				return errors.Errorf("workdir %q does not exist on container %s", workdir, c.ID())
			}
			// This might be a serious error (e.g., permission), so
			// we need to return the full error.
			return errors.Wrapf(err, "error detecting workdir %q on container %s", workdir, c.ID())
		}
		return nil
	}
	if err := os.MkdirAll(resolvedWorkdir, 0755); err != nil {
		if os.IsExist(err) {
			return nil
		}
		return errors.Wrapf(err, "error creating container %s workdir", c.ID())
	}

	// Ensure container entrypoint is created (if required).
	uid, gid, _, err := chrootuser.GetUser(c.state.Mountpoint, c.User())
	if err != nil {
		return errors.Wrapf(err, "error looking up %s inside of the container %s", c.User(), c.ID())
	}
	if err := os.Chown(resolvedWorkdir, int(uid), int(gid)); err != nil {
		return errors.Wrapf(err, "error chowning container %s workdir to container root", c.ID())
	}

	return nil
}

// cleanupNetwork unmounts and cleans up the container's network
func (c *Container) cleanupNetwork() error {
	if c.config.NetNsCtr != "" {
		return nil
	}
	netDisabled, err := c.NetworkDisabled()
	if err != nil {
		return err
	}
	if netDisabled {
		return nil
	}
	if c.state.NetNS == nil {
		logrus.Debugf("Network is already cleaned up, skipping...")
		return nil
	}

	// Stop the container's network namespace (if it has one)
	if err := c.runtime.teardownNetNS(c); err != nil {
		logrus.Errorf("unable to cleanup network for container %s: %q", c.ID(), err)
	}

	c.state.NetNS = nil
	c.state.NetworkStatus = nil

	if c.valid {
		return c.save()
	}

	return nil
}

// reloadNetwork reloads the network for the given container, recreating
// firewall rules.
func (c *Container) reloadNetwork() error {
	result, err := c.runtime.reloadContainerNetwork(c)
	if err != nil {
		return err
	}

	c.state.NetworkStatus = result

	return c.save()
}

func (c *Container) getUserOverrides() *lookup.Overrides {
	var hasPasswdFile, hasGroupFile bool
	overrides := lookup.Overrides{}
	for _, m := range c.config.Spec.Mounts {
		if m.Destination == "/etc/passwd" {
			overrides.ContainerEtcPasswdPath = m.Source
			hasPasswdFile = true
		}
		if m.Destination == "/etc/group" {
			overrides.ContainerEtcGroupPath = m.Source
			hasGroupFile = true
		}
		if m.Destination == "/etc" {
			if !hasPasswdFile {
				overrides.ContainerEtcPasswdPath = filepath.Join(m.Source, "passwd")
			}
			if !hasGroupFile {
				overrides.ContainerEtcGroupPath = filepath.Join(m.Source, "group")
			}
		}
	}
	if path, ok := c.state.BindMounts["/etc/passwd"]; ok {
		overrides.ContainerEtcPasswdPath = path
	}
	return &overrides
}

// Generate spec for a container
// Accepts a map of the container's dependencies
func (c *Container) generateSpec(ctx context.Context) (*spec.Spec, error) {
	overrides := c.getUserOverrides()
	execUser, err := lookup.GetUserGroupInfo(c.state.Mountpoint, c.config.User, overrides)
	if err != nil {
		return nil, err
	}

	g := generate.Generator{Config: c.config.Spec}

	// If network namespace was requested, add it now
	if c.config.CreateNetNS {
		if c.config.PostConfigureNetNS {
			if err := g.AddOrReplaceLinuxNamespace(string(spec.NetworkNamespace), ""); err != nil {
				return nil, err
			}
		} else {
			if err := g.AddOrReplaceLinuxNamespace(string(spec.NetworkNamespace), c.state.NetNS.Path()); err != nil {
				return nil, err
			}
		}
	}

	// Apply AppArmor checks and load the default profile if needed.
	if len(c.config.Spec.Process.ApparmorProfile) > 0 {
		updatedProfile, err := apparmor.CheckProfileAndLoadDefault(c.config.Spec.Process.ApparmorProfile)
		if err != nil {
			return nil, err
		}
		g.SetProcessApparmorProfile(updatedProfile)
	}

	if err := c.makeBindMounts(); err != nil {
		return nil, err
	}

	if err := c.mountNotifySocket(g); err != nil {
		return nil, err
	}

	// Get host UID and GID based on the container process UID and GID.
	hostUID, hostGID, err := butil.GetHostIDs(util.IDtoolsToRuntimeSpec(c.config.IDMappings.UIDMap), util.IDtoolsToRuntimeSpec(c.config.IDMappings.GIDMap), uint32(execUser.Uid), uint32(execUser.Gid))
	if err != nil {
		return nil, err
	}

	// Add named volumes
	for _, namedVol := range c.config.NamedVolumes {
		volume, err := c.runtime.GetVolume(namedVol.Name)
		if err != nil {
			return nil, errors.Wrapf(err, "error retrieving volume %s to add to container %s", namedVol.Name, c.ID())
		}
		mountPoint, err := volume.MountPoint()
		if err != nil {
			return nil, err
		}
		volMount := spec.Mount{
			Type:        "bind",
			Source:      mountPoint,
			Destination: namedVol.Dest,
			Options:     namedVol.Options,
		}
		g.AddMount(volMount)
	}

	// Check if the spec file mounts contain the options z, Z or U.
	// If they have z or Z, relabel the source directory and then remove the option.
	// If they have U, chown the source directory and them remove the option.
	for i := range g.Config.Mounts {
		m := &g.Config.Mounts[i]
		var options []string
		for _, o := range m.Options {
			switch o {
			case "U":
				if m.Type == "tmpfs" {
					options = append(options, []string{fmt.Sprintf("uid=%d", execUser.Uid), fmt.Sprintf("gid=%d", execUser.Gid)}...)
				} else {
					if err := chown.ChangeHostPathOwnership(m.Source, true, int(hostUID), int(hostGID)); err != nil {
						return nil, err
					}
				}
			case "z":
				fallthrough
			case "Z":
				if err := label.Relabel(m.Source, c.MountLabel(), label.IsShared(o)); err != nil {
					return nil, err
				}

			default:
				options = append(options, o)
			}
		}
		m.Options = options
	}

	g.SetProcessSelinuxLabel(c.ProcessLabel())
	g.SetLinuxMountLabel(c.MountLabel())

	// Add bind mounts to container
	for dstPath, srcPath := range c.state.BindMounts {
		newMount := spec.Mount{
			Type:        "bind",
			Source:      srcPath,
			Destination: dstPath,
			Options:     []string{"bind", "rprivate"},
		}
		if c.IsReadOnly() && dstPath != "/dev/shm" {
			newMount.Options = append(newMount.Options, "ro", "nosuid", "noexec", "nodev")
		}
		if !MountExists(g.Mounts(), dstPath) {
			g.AddMount(newMount)
		} else {
			logrus.Infof("User mount overriding libpod mount at %q", dstPath)
		}
	}

	// Add overlay volumes
	for _, overlayVol := range c.config.OverlayVolumes {
		contentDir, err := overlay.TempDir(c.config.StaticDir, c.RootUID(), c.RootGID())
		if err != nil {
			return nil, err
		}
		overlayMount, err := overlay.Mount(contentDir, overlayVol.Source, overlayVol.Dest, c.RootUID(), c.RootGID(), c.runtime.store.GraphOptions())
		if err != nil {
			return nil, errors.Wrapf(err, "mounting overlay failed %q", overlayVol.Source)
		}

		// Check overlay volume options
		for _, o := range overlayVol.Options {
			switch o {
			case "U":
				if err := chown.ChangeHostPathOwnership(overlayVol.Source, true, int(hostUID), int(hostGID)); err != nil {
					return nil, err
				}

				if err := chown.ChangeHostPathOwnership(contentDir, true, int(hostUID), int(hostGID)); err != nil {
					return nil, err
				}
			}
		}

		g.AddMount(overlayMount)
	}

	// Add image volumes as overlay mounts
	for _, volume := range c.config.ImageVolumes {
		// Mount the specified image.
		img, _, err := c.runtime.LibimageRuntime().LookupImage(volume.Source, nil)
		if err != nil {
			return nil, errors.Wrapf(err, "error creating image volume %q:%q", volume.Source, volume.Dest)
		}
		mountPoint, err := img.Mount(ctx, nil, "")
		if err != nil {
			return nil, errors.Wrapf(err, "error mounting image volume %q:%q", volume.Source, volume.Dest)
		}

		contentDir, err := overlay.TempDir(c.config.StaticDir, c.RootUID(), c.RootGID())
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create TempDir in the %s directory", c.config.StaticDir)
		}

		var overlayMount spec.Mount
		if volume.ReadWrite {
			overlayMount, err = overlay.Mount(contentDir, mountPoint, volume.Dest, c.RootUID(), c.RootGID(), c.runtime.store.GraphOptions())
		} else {
			overlayMount, err = overlay.MountReadOnly(contentDir, mountPoint, volume.Dest, c.RootUID(), c.RootGID(), c.runtime.store.GraphOptions())
		}
		if err != nil {
			return nil, errors.Wrapf(err, "creating overlay mount for image %q failed", volume.Source)
		}
		g.AddMount(overlayMount)
	}

	hasHomeSet := false
	for _, s := range c.config.Spec.Process.Env {
		if strings.HasPrefix(s, "HOME=") {
			hasHomeSet = true
			break
		}
	}
	if !hasHomeSet && execUser.Home != "" {
		c.config.Spec.Process.Env = append(c.config.Spec.Process.Env, fmt.Sprintf("HOME=%s", execUser.Home))
	}

	if c.config.User != "" {
		// User and Group must go together
		g.SetProcessUID(uint32(execUser.Uid))
		g.SetProcessGID(uint32(execUser.Gid))
	}

	if c.config.Umask != "" {
		decVal, err := strconv.ParseUint(c.config.Umask, 8, 32)
		if err != nil {
			return nil, errors.Wrapf(err, "Invalid Umask Value")
		}
		umask := uint32(decVal)
		g.Config.Process.User.Umask = &umask
	}

	// Add addition groups if c.config.GroupAdd is not empty
	if len(c.config.Groups) > 0 {
		gids, err := lookup.GetContainerGroups(c.config.Groups, c.state.Mountpoint, overrides)
		if err != nil {
			return nil, errors.Wrapf(err, "error looking up supplemental groups for container %s", c.ID())
		}
		for _, gid := range gids {
			g.AddProcessAdditionalGid(gid)
		}
	}

	if c.config.Systemd {
		if err := c.setupSystemd(g.Mounts(), g); err != nil {
			return nil, errors.Wrapf(err, "error adding systemd-specific mounts")
		}
	}

	// Look up and add groups the user belongs to, if a group wasn't directly specified
	if !strings.Contains(c.config.User, ":") {
		// the gidMappings that are present inside the container user namespace
		var gidMappings []idtools.IDMap

		switch {
		case len(c.config.IDMappings.GIDMap) > 0:
			gidMappings = c.config.IDMappings.GIDMap
		case rootless.IsRootless():
			// Check whether the current user namespace has enough gids available.
			availableGids, err := rootless.GetAvailableGids()
			if err != nil {
				return nil, errors.Wrapf(err, "cannot read number of available GIDs")
			}
			gidMappings = []idtools.IDMap{{
				ContainerID: 0,
				HostID:      0,
				Size:        int(availableGids),
			}}
		default:
			gidMappings = []idtools.IDMap{{
				ContainerID: 0,
				HostID:      0,
				Size:        math.MaxInt32,
			}}
		}
		for _, gid := range execUser.Sgids {
			isGIDAvailable := false
			for _, m := range gidMappings {
				if gid >= m.ContainerID && gid < m.ContainerID+m.Size {
					isGIDAvailable = true
					break
				}
			}
			if isGIDAvailable {
				g.AddProcessAdditionalGid(uint32(gid))
			} else {
				logrus.Warnf("additional gid=%d is not present in the user namespace, skip setting it", gid)
			}
		}
	}

	// Add shared namespaces from other containers
	if c.config.IPCNsCtr != "" {
		if err := c.addNamespaceContainer(&g, IPCNS, c.config.IPCNsCtr, spec.IPCNamespace); err != nil {
			return nil, err
		}
	}
	if c.config.MountNsCtr != "" {
		if err := c.addNamespaceContainer(&g, MountNS, c.config.MountNsCtr, spec.MountNamespace); err != nil {
			return nil, err
		}
	}
	if c.config.NetNsCtr != "" {
		if err := c.addNamespaceContainer(&g, NetNS, c.config.NetNsCtr, spec.NetworkNamespace); err != nil {
			return nil, err
		}
	}
	if c.config.PIDNsCtr != "" {
		if err := c.addNamespaceContainer(&g, PIDNS, c.config.PIDNsCtr, spec.PIDNamespace); err != nil {
			return nil, err
		}
	}
	if c.config.UserNsCtr != "" {
		if err := c.addNamespaceContainer(&g, UserNS, c.config.UserNsCtr, spec.UserNamespace); err != nil {
			return nil, err
		}
		if len(g.Config.Linux.UIDMappings) == 0 {
			// runc complains if no mapping is specified, even if we join another ns.  So provide a dummy mapping
			g.AddLinuxUIDMapping(uint32(0), uint32(0), uint32(1))
			g.AddLinuxGIDMapping(uint32(0), uint32(0), uint32(1))
		}
	}

	availableUIDs, availableGIDs, err := rootless.GetAvailableIDMaps()
	if err != nil {
		if os.IsNotExist(err) {
			// The kernel-provided files only exist if user namespaces are supported
			logrus.Debugf("user or group ID mappings not available: %s", err)
		} else {
			return nil, err
		}
	} else {
		g.Config.Linux.UIDMappings = rootless.MaybeSplitMappings(g.Config.Linux.UIDMappings, availableUIDs)
		g.Config.Linux.GIDMappings = rootless.MaybeSplitMappings(g.Config.Linux.GIDMappings, availableGIDs)
	}

	// Hostname handling:
	// If we have a UTS namespace, set Hostname in the OCI spec.
	// Set the HOSTNAME environment variable unless explicitly overridden by
	// the user (already present in OCI spec). If we don't have a UTS ns,
	// set it to the host's hostname instead.
	hostname := c.Hostname()
	foundUTS := false

	for _, i := range c.config.Spec.Linux.Namespaces {
		if i.Type == spec.UTSNamespace && i.Path == "" {
			foundUTS = true
			g.SetHostname(hostname)
			break
		}
	}
	if !foundUTS {
		tmpHostname, err := os.Hostname()
		if err != nil {
			return nil, err
		}
		hostname = tmpHostname
	}
	needEnv := true
	for _, checkEnv := range g.Config.Process.Env {
		if strings.SplitN(checkEnv, "=", 2)[0] == "HOSTNAME" {
			needEnv = false
			break
		}
	}
	if needEnv {
		g.AddProcessEnv("HOSTNAME", hostname)
	}

	if c.config.UTSNsCtr != "" {
		if err := c.addNamespaceContainer(&g, UTSNS, c.config.UTSNsCtr, spec.UTSNamespace); err != nil {
			return nil, err
		}
	}
	if c.config.CgroupNsCtr != "" {
		if err := c.addNamespaceContainer(&g, CgroupNS, c.config.CgroupNsCtr, spec.CgroupNamespace); err != nil {
			return nil, err
		}
	}

	if c.config.UserNsCtr == "" && c.config.IDMappings.AutoUserNs {
		if err := g.AddOrReplaceLinuxNamespace(string(spec.UserNamespace), ""); err != nil {
			return nil, err
		}
		g.ClearLinuxUIDMappings()
		for _, uidmap := range c.config.IDMappings.UIDMap {
			g.AddLinuxUIDMapping(uint32(uidmap.HostID), uint32(uidmap.ContainerID), uint32(uidmap.Size))
		}
		g.ClearLinuxGIDMappings()
		for _, gidmap := range c.config.IDMappings.GIDMap {
			g.AddLinuxGIDMapping(uint32(gidmap.HostID), uint32(gidmap.ContainerID), uint32(gidmap.Size))
		}
	}

	g.SetRootPath(c.state.Mountpoint)
	g.AddAnnotation(annotations.Created, c.config.CreatedTime.Format(time.RFC3339Nano))
	g.AddAnnotation("org.opencontainers.image.stopSignal", fmt.Sprintf("%d", c.config.StopSignal))

	if _, exists := g.Config.Annotations[annotations.ContainerManager]; !exists {
		g.AddAnnotation(annotations.ContainerManager, annotations.ContainerManagerLibpod)
	}

	// Only add container environment variable if not already present
	foundContainerEnv := false
	for _, env := range g.Config.Process.Env {
		if strings.HasPrefix(env, "container=") {
			foundContainerEnv = true
			break
		}
	}
	if !foundContainerEnv {
		g.AddProcessEnv("container", "libpod")
	}

	cgroupPath, err := c.getOCICgroupPath()
	if err != nil {
		return nil, err
	}
	g.SetLinuxCgroupsPath(cgroupPath)

	// Warning: CDI may alter g.Config in place.
	if len(c.config.CDIDevices) > 0 {
		if err = cdi.UpdateOCISpecForDevices(g.Config, c.config.CDIDevices); err != nil {
			return nil, errors.Wrapf(err, "error setting up CDI devices")
		}
	}

	// Mounts need to be sorted so paths will not cover other paths
	mounts := sortMounts(g.Mounts())
	g.ClearMounts()

	// Determine property of RootPropagation based on volume properties. If
	// a volume is shared, then keep root propagation shared. This should
	// work for slave and private volumes too.
	//
	// For slave volumes, it can be either [r]shared/[r]slave.
	//
	// For private volumes any root propagation value should work.
	rootPropagation := ""
	for _, m := range mounts {
		// We need to remove all symlinks from tmpfs mounts.
		// Runc and other runtimes may choke on them.
		// Easy solution: use securejoin to do a scoped evaluation of
		// the links, then trim off the mount prefix.
		if m.Type == "tmpfs" {
			finalPath, err := securejoin.SecureJoin(c.state.Mountpoint, m.Destination)
			if err != nil {
				return nil, errors.Wrapf(err, "error resolving symlinks for mount destination %s", m.Destination)
			}
			trimmedPath := strings.TrimPrefix(finalPath, strings.TrimSuffix(c.state.Mountpoint, "/"))
			m.Destination = trimmedPath
		}
		g.AddMount(m)
		for _, opt := range m.Options {
			switch opt {
			case MountShared, MountRShared:
				if rootPropagation != MountShared && rootPropagation != MountRShared {
					rootPropagation = MountShared
				}
			case MountSlave, MountRSlave:
				if rootPropagation != MountShared && rootPropagation != MountRShared && rootPropagation != MountSlave && rootPropagation != MountRSlave {
					rootPropagation = MountRSlave
				}
			}
		}
	}

	if rootPropagation != "" {
		logrus.Debugf("set root propagation to %q", rootPropagation)
		if err := g.SetLinuxRootPropagation(rootPropagation); err != nil {
			return nil, err
		}
	}

	// Warning: precreate hooks may alter g.Config in place.
	if c.state.ExtensionStageHooks, err = c.setupOCIHooks(ctx, g.Config); err != nil {
		return nil, errors.Wrapf(err, "error setting up OCI Hooks")
	}
	if len(c.config.EnvSecrets) > 0 {
		manager, err := c.runtime.SecretsManager()
		if err != nil {
			return nil, err
		}
		if err != nil {
			return nil, err
		}
		for name, secr := range c.config.EnvSecrets {
			_, data, err := manager.LookupSecretData(secr.Name)
			if err != nil {
				return nil, err
			}
			g.AddProcessEnv(name, string(data))
		}
	}

	// Pass down the LISTEN_* environment (see #10443).
	for _, key := range []string{"LISTEN_PID", "LISTEN_FDS", "LISTEN_FDNAMES"} {
		if val, ok := os.LookupEnv(key); ok {
			// Force the PID to `1` since we cannot rely on (all
			// versions of) all runtimes to do it for us.
			if key == "LISTEN_PID" {
				val = "1"
			}
			g.AddProcessEnv(key, val)
		}
	}

	return g.Config, nil
}

// mountNotifySocket mounts the NOTIFY_SOCKET into the container if it's set
// and if the sdnotify mode is set to container.  It also sets c.notifySocket
// to avoid redundantly looking up the env variable.
func (c *Container) mountNotifySocket(g generate.Generator) error {
	notify, ok := os.LookupEnv("NOTIFY_SOCKET")
	if !ok {
		return nil
	}
	c.notifySocket = notify

	if c.config.SdNotifyMode != define.SdNotifyModeContainer {
		return nil
	}

	notifyDir := filepath.Join(c.bundlePath(), "notify")
	logrus.Debugf("checking notify %q dir", notifyDir)
	if err := os.MkdirAll(notifyDir, 0755); err != nil {
		if !os.IsExist(err) {
			return errors.Wrapf(err, "unable to create notify %q dir", notifyDir)
		}
	}
	if err := label.Relabel(notifyDir, c.MountLabel(), true); err != nil {
		return errors.Wrapf(err, "relabel failed %q", notifyDir)
	}
	logrus.Debugf("add bindmount notify %q dir", notifyDir)
	if _, ok := c.state.BindMounts["/run/notify"]; !ok {
		c.state.BindMounts["/run/notify"] = notifyDir
	}

	// Set the container's notify socket to the proxy socket created by conmon
	g.AddProcessEnv("NOTIFY_SOCKET", "/run/notify/notify.sock")

	return nil
}

// systemd expects to have /run, /run/lock and /tmp on tmpfs
// It also expects to be able to write to /sys/fs/cgroup/systemd and /var/log/journal
func (c *Container) setupSystemd(mounts []spec.Mount, g generate.Generator) error {
	options := []string{"rw", "rprivate", "nosuid", "nodev"}
	for _, dest := range []string{"/run", "/run/lock"} {
		if MountExists(mounts, dest) {
			continue
		}
		tmpfsMnt := spec.Mount{
			Destination: dest,
			Type:        "tmpfs",
			Source:      "tmpfs",
			Options:     append(options, "tmpcopyup"),
		}
		g.AddMount(tmpfsMnt)
	}
	for _, dest := range []string{"/tmp", "/var/log/journal"} {
		if MountExists(mounts, dest) {
			continue
		}
		tmpfsMnt := spec.Mount{
			Destination: dest,
			Type:        "tmpfs",
			Source:      "tmpfs",
			Options:     append(options, "tmpcopyup"),
		}
		g.AddMount(tmpfsMnt)
	}

	unified, err := cgroups.IsCgroup2UnifiedMode()
	if err != nil {
		return err
	}

	if unified {
		g.RemoveMount("/sys/fs/cgroup")

		hasCgroupNs := false
		for _, ns := range c.config.Spec.Linux.Namespaces {
			if ns.Type == spec.CgroupNamespace {
				hasCgroupNs = true
				break
			}
		}

		var systemdMnt spec.Mount
		if hasCgroupNs {
			systemdMnt = spec.Mount{
				Destination: "/sys/fs/cgroup",
				Type:        "cgroup",
				Source:      "cgroup",
				Options:     []string{"private", "rw"},
			}
		} else {
			systemdMnt = spec.Mount{
				Destination: "/sys/fs/cgroup",
				Type:        "bind",
				Source:      "/sys/fs/cgroup",
				Options:     []string{"bind", "private", "rw"},
			}
		}
		g.AddMount(systemdMnt)
	} else {
		mountOptions := []string{"bind", "rprivate"}

		var statfs unix.Statfs_t
		if err := unix.Statfs("/sys/fs/cgroup/systemd", &statfs); err != nil {
			mountOptions = append(mountOptions, "nodev", "noexec", "nosuid")
		} else {
			if statfs.Flags&unix.MS_NODEV == unix.MS_NODEV {
				mountOptions = append(mountOptions, "nodev")
			}
			if statfs.Flags&unix.MS_NOEXEC == unix.MS_NOEXEC {
				mountOptions = append(mountOptions, "noexec")
			}
			if statfs.Flags&unix.MS_NOSUID == unix.MS_NOSUID {
				mountOptions = append(mountOptions, "nosuid")
			}
			if statfs.Flags&unix.MS_RDONLY == unix.MS_RDONLY {
				mountOptions = append(mountOptions, "ro")
			}
		}

		systemdMnt := spec.Mount{
			Destination: "/sys/fs/cgroup/systemd",
			Type:        "bind",
			Source:      "/sys/fs/cgroup/systemd",
			Options:     mountOptions,
		}
		g.AddMount(systemdMnt)
		g.AddLinuxMaskedPaths("/sys/fs/cgroup/systemd/release_agent")
	}

	return nil
}

// Add an existing container's namespace to the spec
func (c *Container) addNamespaceContainer(g *generate.Generator, ns LinuxNS, ctr string, specNS spec.LinuxNamespaceType) error {
	nsCtr, err := c.runtime.state.Container(ctr)
	if err != nil {
		return errors.Wrapf(err, "error retrieving dependency %s of container %s from state", ctr, c.ID())
	}

	if specNS == spec.UTSNamespace {
		hostname := nsCtr.Hostname()
		// Joining an existing namespace, cannot set the hostname
		g.SetHostname("")
		g.AddProcessEnv("HOSTNAME", hostname)
	}

	// TODO need unlocked version of this for use in pods
	nsPath, err := nsCtr.NamespacePath(ns)
	if err != nil {
		return err
	}

	if err := g.AddOrReplaceLinuxNamespace(string(specNS), nsPath); err != nil {
		return err
	}

	return nil
}

func (c *Container) exportCheckpoint(options ContainerCheckpointOptions) error {
	if len(c.Dependencies()) == 1 {
		// Check if the dependency is an infra container. If it is we can checkpoint
		// the container out of the Pod.
		if c.config.Pod == "" {
			return errors.Errorf("cannot export checkpoints of containers with dependencies")
		}

		pod, err := c.runtime.state.Pod(c.config.Pod)
		if err != nil {
			return errors.Wrapf(err, "container %s is in pod %s, but pod cannot be retrieved", c.ID(), c.config.Pod)
		}
		infraID, err := pod.InfraContainerID()
		if err != nil {
			return errors.Wrapf(err, "cannot retrieve infra container ID for pod %s", c.config.Pod)
		}
		if c.Dependencies()[0] != infraID {
			return errors.Errorf("cannot export checkpoints of containers with dependencies")
		}
	}
	if len(c.Dependencies()) > 1 {
		return errors.Errorf("cannot export checkpoints of containers with dependencies")
	}
	logrus.Debugf("Exporting checkpoint image of container %q to %q", c.ID(), options.TargetFile)

	includeFiles := []string{
		"artifacts",
		metadata.ConfigDumpFile,
		metadata.SpecDumpFile,
		metadata.NetworkStatusFile,
	}

	if c.LogDriver() == define.KubernetesLogging ||
		c.LogDriver() == define.JSONLogging {
		includeFiles = append(includeFiles, "ctr.log")
	}
	if options.PreCheckPoint {
		includeFiles = append(includeFiles, preCheckpointDir)
	} else {
		includeFiles = append(includeFiles, metadata.CheckpointDirectory)
	}
	// Get root file-system changes included in the checkpoint archive
	var addToTarFiles []string
	if !options.IgnoreRootfs {
		// To correctly track deleted files, let's go through the output of 'podman diff'
		rootFsChanges, err := c.runtime.GetDiff("", c.ID(), define.DiffContainer)
		if err != nil {
			return errors.Wrapf(err, "error exporting root file-system diff for %q", c.ID())
		}

		addToTarFiles, err := crutils.CRCreateRootFsDiffTar(&rootFsChanges, c.state.Mountpoint, c.bundlePath())
		if err != nil {
			return err
		}

		includeFiles = append(includeFiles, addToTarFiles...)
	}

	// Folder containing archived volumes that will be included in the export
	expVolDir := filepath.Join(c.bundlePath(), "volumes")

	// Create an archive for each volume associated with the container
	if !options.IgnoreVolumes {
		if err := os.MkdirAll(expVolDir, 0700); err != nil {
			return errors.Wrapf(err, "error creating volumes export directory %q", expVolDir)
		}

		for _, v := range c.config.NamedVolumes {
			volumeTarFilePath := filepath.Join("volumes", v.Name+".tar")
			volumeTarFileFullPath := filepath.Join(c.bundlePath(), volumeTarFilePath)

			volumeTarFile, err := os.Create(volumeTarFileFullPath)
			if err != nil {
				return errors.Wrapf(err, "error creating %q", volumeTarFileFullPath)
			}

			volume, err := c.runtime.GetVolume(v.Name)
			if err != nil {
				return err
			}

			mp, err := volume.MountPoint()
			if err != nil {
				return err
			}
			if mp == "" {
				return errors.Wrapf(define.ErrInternal, "volume %s is not mounted, cannot export", volume.Name())
			}

			input, err := archive.TarWithOptions(mp, &archive.TarOptions{
				Compression:      archive.Uncompressed,
				IncludeSourceDir: true,
			})
			if err != nil {
				return errors.Wrapf(err, "error reading volume directory %q", v.Dest)
			}

			_, err = io.Copy(volumeTarFile, input)
			if err != nil {
				return err
			}
			volumeTarFile.Close()

			includeFiles = append(includeFiles, volumeTarFilePath)
		}
	}

	input, err := archive.TarWithOptions(c.bundlePath(), &archive.TarOptions{
		Compression:      options.Compression,
		IncludeSourceDir: true,
		IncludeFiles:     includeFiles,
	})

	if err != nil {
		return errors.Wrapf(err, "error reading checkpoint directory %q", c.ID())
	}

	outFile, err := os.Create(options.TargetFile)
	if err != nil {
		return errors.Wrapf(err, "error creating checkpoint export file %q", options.TargetFile)
	}
	defer outFile.Close()

	if err := os.Chmod(options.TargetFile, 0600); err != nil {
		return err
	}

	_, err = io.Copy(outFile, input)
	if err != nil {
		return err
	}

	for _, file := range addToTarFiles {
		os.Remove(filepath.Join(c.bundlePath(), file))
	}

	if !options.IgnoreVolumes {
		os.RemoveAll(expVolDir)
	}

	return nil
}

func (c *Container) checkpointRestoreSupported(version int) error {
	if !criu.CheckForCriu(version) {
		return errors.Errorf("checkpoint/restore requires at least CRIU %d", version)
	}
	if !c.ociRuntime.SupportsCheckpoint() {
		return errors.Errorf("configured runtime does not support checkpoint/restore")
	}
	return nil
}

func (c *Container) checkpoint(ctx context.Context, options ContainerCheckpointOptions) error {
	if err := c.checkpointRestoreSupported(criu.MinCriuVersion); err != nil {
		return err
	}

	if c.state.State != define.ContainerStateRunning {
		return errors.Wrapf(define.ErrCtrStateInvalid, "%q is not running, cannot checkpoint", c.state.State)
	}

	if c.AutoRemove() && options.TargetFile == "" {
		return errors.Errorf("cannot checkpoint containers that have been started with '--rm' unless '--export' is used")
	}

	if err := crutils.CRCreateFileWithLabel(c.bundlePath(), "dump.log", c.MountLabel()); err != nil {
		return err
	}

	if err := c.ociRuntime.CheckpointContainer(c, options); err != nil {
		return err
	}

	// Save network.status. This is needed to restore the container with
	// the same IP. Currently limited to one IP address in a container
	// with one interface.
	if _, err := metadata.WriteJSONFile(c.state.NetworkStatus, c.bundlePath(), metadata.NetworkStatusFile); err != nil {
		return err
	}

	defer c.newContainerEvent(events.Checkpoint)

	// There is a bug from criu: https://github.com/checkpoint-restore/criu/issues/116
	// We have to change the symbolic link from absolute path to relative path
	if options.WithPrevious {
		os.Remove(path.Join(c.CheckpointPath(), "parent"))
		if err := os.Symlink("../pre-checkpoint", path.Join(c.CheckpointPath(), "parent")); err != nil {
			return err
		}
	}

	if options.TargetFile != "" {
		if err := c.exportCheckpoint(options); err != nil {
			return err
		}
	}

	logrus.Debugf("Checkpointed container %s", c.ID())

	if !options.KeepRunning && !options.PreCheckPoint {
		c.state.State = define.ContainerStateStopped
		c.state.Checkpointed = true

		// Cleanup Storage and Network
		if err := c.cleanup(ctx); err != nil {
			return err
		}
	}

	if !options.Keep && !options.PreCheckPoint {
		cleanup := []string{
			"dump.log",
			"stats-dump",
			metadata.ConfigDumpFile,
			metadata.SpecDumpFile,
		}
		for _, del := range cleanup {
			file := filepath.Join(c.bundlePath(), del)
			if err := os.Remove(file); err != nil {
				logrus.Debugf("unable to remove file %s", file)
			}
		}
	}

	c.state.FinishedTime = time.Now()
	return c.save()
}

func (c *Container) importCheckpoint(input string) error {
	if err := crutils.CRImportCheckpointWithoutConfig(c.bundlePath(), input); err != nil {
		return err
	}

	// Make sure the newly created config.json exists on disk
	g := generate.Generator{Config: c.config.Spec}
	if err := c.saveSpec(g.Config); err != nil {
		return errors.Wrap(err, "saving imported container specification for restore failed")
	}

	return nil
}

func (c *Container) importPreCheckpoint(input string) error {
	archiveFile, err := os.Open(input)
	if err != nil {
		return errors.Wrap(err, "failed to open pre-checkpoint archive for import")
	}

	defer archiveFile.Close()

	err = archive.Untar(archiveFile, c.bundlePath(), nil)
	if err != nil {
		return errors.Wrapf(err, "Unpacking of pre-checkpoint archive %s failed", input)
	}
	return nil
}

func (c *Container) restore(ctx context.Context, options ContainerCheckpointOptions) (retErr error) {
	minCriuVersion := func() int {
		if options.Pod == "" {
			return criu.MinCriuVersion
		}
		return criu.PodCriuVersion
	}()
	if err := c.checkpointRestoreSupported(minCriuVersion); err != nil {
		return err
	}

	if options.Pod != "" && !crutils.CRRuntimeSupportsPodCheckpointRestore(c.ociRuntime.Path()) {
		return errors.Errorf("runtime %s does not support pod restore", c.ociRuntime.Path())
	}

	if !c.ensureState(define.ContainerStateConfigured, define.ContainerStateExited) {
		return errors.Wrapf(define.ErrCtrStateInvalid, "container %s is running or paused, cannot restore", c.ID())
	}

	if options.ImportPrevious != "" {
		if err := c.importPreCheckpoint(options.ImportPrevious); err != nil {
			return err
		}
	}

	if options.TargetFile != "" {
		if err := c.importCheckpoint(options.TargetFile); err != nil {
			return err
		}
	}

	// Let's try to stat() CRIU's inventory file. If it does not exist, it makes
	// no sense to try a restore. This is a minimal check if a checkpoint exist.
	if _, err := os.Stat(filepath.Join(c.CheckpointPath(), "inventory.img")); os.IsNotExist(err) {
		return errors.Wrapf(err, "a complete checkpoint for this container cannot be found, cannot restore")
	}

	if err := crutils.CRCreateFileWithLabel(c.bundlePath(), "restore.log", c.MountLabel()); err != nil {
		return err
	}

	// If a container is restored multiple times from an exported checkpoint with
	// the help of '--import --name', the restore will fail if during 'podman run'
	// a static container IP was set with '--ip'. The user can tell the restore
	// process to ignore the static IP with '--ignore-static-ip'
	if options.IgnoreStaticIP {
		c.config.StaticIP = nil
	}

	// If a container is restored multiple times from an exported checkpoint with
	// the help of '--import --name', the restore will fail if during 'podman run'
	// a static container MAC address was set with '--mac-address'. The user
	// can tell the restore process to ignore the static MAC with
	// '--ignore-static-mac'
	if options.IgnoreStaticMAC {
		c.config.StaticMAC = nil
	}

	// Read network configuration from checkpoint
	// Currently only one interface with one IP is supported.
	networkStatus, _, err := metadata.ReadContainerCheckpointNetworkStatus(c.bundlePath())
	// If the restored container should get a new name, the IP address of
	// the container will not be restored. This assumes that if a new name is
	// specified, the container is restored multiple times.
	// TODO: This implicit restoring with or without IP depending on an
	//       unrelated restore parameter (--name) does not seem like the
	//       best solution.
	if err == nil && options.Name == "" && (!options.IgnoreStaticIP || !options.IgnoreStaticMAC) {
		// The file with the network.status does exist. Let's restore the
		// container with the same IP address / MAC address as during checkpointing.
		if !options.IgnoreStaticIP {
			if IP := metadata.GetIPFromNetworkStatus(networkStatus); IP != nil {
				// Tell CNI which IP address we want.
				c.requestedIP = IP
			}
		}
		if !options.IgnoreStaticMAC {
			if MAC := metadata.GetMACFromNetworkStatus(networkStatus); MAC != nil {
				// Tell CNI which MAC address we want.
				c.requestedMAC = MAC
			}
		}
	}

	defer func() {
		if retErr != nil {
			if err := c.cleanup(ctx); err != nil {
				logrus.Errorf("error cleaning up container %s: %v", c.ID(), err)
			}
		}
	}()

	if err := c.prepare(); err != nil {
		return err
	}

	// Read config
	jsonPath := filepath.Join(c.bundlePath(), "config.json")
	logrus.Debugf("generate.NewFromFile at %v", jsonPath)
	g, err := generate.NewFromFile(jsonPath)
	if err != nil {
		logrus.Debugf("generate.NewFromFile failed with %v", err)
		return err
	}

	// Restoring from an import means that we are doing migration
	if options.TargetFile != "" {
		g.SetRootPath(c.state.Mountpoint)
	}

	// We want to have the same network namespace as before.
	if c.config.CreateNetNS {
		netNSPath := ""
		if !c.config.PostConfigureNetNS {
			netNSPath = c.state.NetNS.Path()
		}

		if err := g.AddOrReplaceLinuxNamespace(string(spec.NetworkNamespace), netNSPath); err != nil {
			return err
		}
	}

	if options.Pod != "" {
		// Running in a Pod means that we have to change all namespace settings to
		// the ones from the infrastructure container.
		pod, err := c.runtime.LookupPod(options.Pod)
		if err != nil {
			return errors.Wrapf(err, "pod %q cannot be retrieved", options.Pod)
		}

		infraContainer, err := pod.InfraContainer()
		if err != nil {
			return errors.Wrapf(err, "cannot retrieved infra container from pod %q", options.Pod)
		}

		infraContainer.lock.Lock()
		if err := infraContainer.syncContainer(); err != nil {
			infraContainer.lock.Unlock()
			return errors.Wrapf(err, "Error syncing infrastructure container %s status", infraContainer.ID())
		}
		if infraContainer.state.State != define.ContainerStateRunning {
			if err := infraContainer.initAndStart(ctx); err != nil {
				infraContainer.lock.Unlock()
				return errors.Wrapf(err, "Error starting infrastructure container %s status", infraContainer.ID())
			}
		}
		infraContainer.lock.Unlock()

		if c.config.IPCNsCtr != "" {
			nsPath, err := infraContainer.namespacePath(IPCNS)
			if err != nil {
				return errors.Wrapf(err, "cannot retrieve IPC namespace path for Pod %q", options.Pod)
			}
			if err := g.AddOrReplaceLinuxNamespace(string(spec.IPCNamespace), nsPath); err != nil {
				return err
			}
		}

		if c.config.NetNsCtr != "" {
			nsPath, err := infraContainer.namespacePath(NetNS)
			if err != nil {
				return errors.Wrapf(err, "cannot retrieve network namespace path for Pod %q", options.Pod)
			}
			if err := g.AddOrReplaceLinuxNamespace(string(spec.NetworkNamespace), nsPath); err != nil {
				return err
			}
		}

		if c.config.PIDNsCtr != "" {
			nsPath, err := infraContainer.namespacePath(PIDNS)
			if err != nil {
				return errors.Wrapf(err, "cannot retrieve PID namespace path for Pod %q", options.Pod)
			}
			if err := g.AddOrReplaceLinuxNamespace(string(spec.PIDNamespace), nsPath); err != nil {
				return err
			}
		}

		if c.config.UTSNsCtr != "" {
			nsPath, err := infraContainer.namespacePath(UTSNS)
			if err != nil {
				return errors.Wrapf(err, "cannot retrieve UTS namespace path for Pod %q", options.Pod)
			}
			if err := g.AddOrReplaceLinuxNamespace(string(spec.UTSNamespace), nsPath); err != nil {
				return err
			}
		}

		if c.config.CgroupNsCtr != "" {
			nsPath, err := infraContainer.namespacePath(CgroupNS)
			if err != nil {
				return errors.Wrapf(err, "cannot retrieve Cgroup namespace path for Pod %q", options.Pod)
			}
			if err := g.AddOrReplaceLinuxNamespace(string(spec.CgroupNamespace), nsPath); err != nil {
				return err
			}
		}
	}

	if err := c.makeBindMounts(); err != nil {
		return err
	}

	if options.TargetFile != "" {
		for dstPath, srcPath := range c.state.BindMounts {
			newMount := spec.Mount{
				Type:        "bind",
				Source:      srcPath,
				Destination: dstPath,
				Options:     []string{"bind", "private"},
			}
			if c.IsReadOnly() && dstPath != "/dev/shm" {
				newMount.Options = append(newMount.Options, "ro", "nosuid", "noexec", "nodev")
			}
			if !MountExists(g.Mounts(), dstPath) {
				g.AddMount(newMount)
			}
		}
	}

	// Cleanup for a working restore.
	if err := c.removeConmonFiles(); err != nil {
		return err
	}

	// Save the OCI spec to disk
	if err := c.saveSpec(g.Config); err != nil {
		return err
	}

	// When restoring from an imported archive, allow restoring the content of volumes.
	// Volumes are created in setupContainer()
	if options.TargetFile != "" && !options.IgnoreVolumes {
		for _, v := range c.config.NamedVolumes {
			volumeFilePath := filepath.Join(c.bundlePath(), "volumes", v.Name+".tar")

			volumeFile, err := os.Open(volumeFilePath)
			if err != nil {
				return errors.Wrapf(err, "failed to open volume file %s", volumeFilePath)
			}
			defer volumeFile.Close()

			volume, err := c.runtime.GetVolume(v.Name)
			if err != nil {
				return errors.Wrapf(err, "failed to retrieve volume %s", v.Name)
			}

			mountPoint, err := volume.MountPoint()
			if err != nil {
				return err
			}
			if mountPoint == "" {
				return errors.Wrapf(err, "unable to import volume %s as it is not mounted", volume.Name())
			}
			if err := archive.UntarUncompressed(volumeFile, mountPoint, nil); err != nil {
				return errors.Wrapf(err, "Failed to extract volume %s to %s", volumeFilePath, mountPoint)
			}
		}
	}

	// Before actually restarting the container, apply the root file-system changes
	if !options.IgnoreRootfs {
		if err := crutils.CRApplyRootFsDiffTar(c.bundlePath(), c.state.Mountpoint); err != nil {
			return err
		}

		if err := crutils.CRRemoveDeletedFiles(c.ID(), c.bundlePath(), c.state.Mountpoint); err != nil {
			return err
		}
	}

	if err := c.ociRuntime.CreateContainer(c, &options); err != nil {
		return err
	}

	logrus.Debugf("Restored container %s", c.ID())

	c.state.State = define.ContainerStateRunning
	c.state.Checkpointed = false

	if !options.Keep {
		// Delete all checkpoint related files. At this point, in theory, all files
		// should exist. Still ignoring errors for now as the container should be
		// restored and running. Not erroring out just because some cleanup operation
		// failed. Starting with the checkpoint directory
		err = os.RemoveAll(c.CheckpointPath())
		if err != nil {
			logrus.Debugf("Non-fatal: removal of checkpoint directory (%s) failed: %v", c.CheckpointPath(), err)
		}
		err = os.RemoveAll(c.PreCheckPointPath())
		if err != nil {
			logrus.Debugf("Non-fatal: removal of pre-checkpoint directory (%s) failed: %v", c.PreCheckPointPath(), err)
		}
		cleanup := [...]string{
			"restore.log",
			"dump.log",
			"stats-dump",
			"stats-restore",
			metadata.NetworkStatusFile,
			metadata.RootFsDiffTar,
			metadata.DeletedFilesFile,
		}
		for _, del := range cleanup {
			file := filepath.Join(c.bundlePath(), del)
			err = os.Remove(file)
			if err != nil {
				logrus.Debugf("Non-fatal: removal of checkpoint file (%s) failed: %v", file, err)
			}
		}
	}

	return c.save()
}

// Retrieves a container's "root" net namespace container dependency.
func (c *Container) getRootNetNsDepCtr() (depCtr *Container, err error) {
	containersVisited := map[string]int{c.config.ID: 1}
	nextCtr := c.config.NetNsCtr
	for nextCtr != "" {
		// Make sure we aren't in a loop
		if _, visited := containersVisited[nextCtr]; visited {
			return nil, errors.New("loop encountered while determining net namespace container")
		}
		containersVisited[nextCtr] = 1

		depCtr, err = c.runtime.state.Container(nextCtr)
		if err != nil {
			return nil, errors.Wrapf(err, "error fetching dependency %s of container %s", c.config.NetNsCtr, c.ID())
		}
		// This should never happen without an error
		if depCtr == nil {
			break
		}
		nextCtr = depCtr.config.NetNsCtr
	}

	if depCtr == nil {
		return nil, errors.New("unexpected error depCtr is nil without reported error from runtime state")
	}
	return depCtr, nil
}

// Make standard bind mounts to include in the container
func (c *Container) makeBindMounts() error {
	if err := os.Chown(c.state.RunDir, c.RootUID(), c.RootGID()); err != nil {
		return errors.Wrap(err, "cannot chown run directory")
	}

	if c.state.BindMounts == nil {
		c.state.BindMounts = make(map[string]string)
	}
	netDisabled, err := c.NetworkDisabled()
	if err != nil {
		return err
	}

	if !netDisabled {
		// If /etc/resolv.conf and /etc/hosts exist, delete them so we
		// will recreate. Only do this if we aren't sharing them with
		// another container.
		if c.config.NetNsCtr == "" {
			if resolvePath, ok := c.state.BindMounts["/etc/resolv.conf"]; ok {
				if err := os.Remove(resolvePath); err != nil && !os.IsNotExist(err) {
					return errors.Wrapf(err, "container %s", c.ID())
				}
				delete(c.state.BindMounts, "/etc/resolv.conf")
			}
			if hostsPath, ok := c.state.BindMounts["/etc/hosts"]; ok {
				if err := os.Remove(hostsPath); err != nil && !os.IsNotExist(err) {
					return errors.Wrapf(err, "container %s", c.ID())
				}
				delete(c.state.BindMounts, "/etc/hosts")
			}
		}

		if c.config.NetNsCtr != "" && (!c.config.UseImageResolvConf || !c.config.UseImageHosts) {
			// We share a net namespace.
			// We want /etc/resolv.conf and /etc/hosts from the
			// other container. Unless we're not creating both of
			// them.
			depCtr, err := c.getRootNetNsDepCtr()
			if err != nil {
				return errors.Wrapf(err, "error fetching network namespace dependency container for container %s", c.ID())
			}

			// We need that container's bind mounts
			bindMounts, err := depCtr.BindMounts()
			if err != nil {
				return errors.Wrapf(err, "error fetching bind mounts from dependency %s of container %s", depCtr.ID(), c.ID())
			}

			// The other container may not have a resolv.conf or /etc/hosts
			// If it doesn't, don't copy them
			resolvPath, exists := bindMounts["/etc/resolv.conf"]
			if !c.config.UseImageResolvConf && exists {
				c.state.BindMounts["/etc/resolv.conf"] = resolvPath
			}

			// check if dependency container has an /etc/hosts file.
			// It may not have one, so only use it if it does.
			hostsPath, exists := bindMounts["/etc/hosts"]
			if !c.config.UseImageHosts && exists {
				depCtr.lock.Lock()
				// generate a hosts file for the dependency container,
				// based on either its old hosts file, or the default,
				// and add the relevant information from the new container (hosts and IP)
				hostsPath, err = depCtr.appendHosts(hostsPath, c)

				if err != nil {
					depCtr.lock.Unlock()
					return errors.Wrapf(err, "error creating hosts file for container %s which depends on container %s", c.ID(), depCtr.ID())
				}
				depCtr.lock.Unlock()

				// finally, save it in the new container
				c.state.BindMounts["/etc/hosts"] = hostsPath
			}

			if !hasCurrentUserMapped(c) {
				if err := makeAccessible(resolvPath, c.RootUID(), c.RootGID()); err != nil {
					return err
				}
				if err := makeAccessible(hostsPath, c.RootUID(), c.RootGID()); err != nil {
					return err
				}
			}
		} else {
			if !c.config.UseImageResolvConf {
				newResolv, err := c.generateResolvConf()
				if err != nil {
					return errors.Wrapf(err, "error creating resolv.conf for container %s", c.ID())
				}
				c.state.BindMounts["/etc/resolv.conf"] = newResolv
			}

			if !c.config.UseImageHosts {
				newHosts, err := c.generateHosts("/etc/hosts")
				if err != nil {
					return errors.Wrapf(err, "error creating hosts file for container %s", c.ID())
				}
				c.state.BindMounts["/etc/hosts"] = newHosts
			}
		}

		if c.state.BindMounts["/etc/hosts"] != "" {
			if err := label.Relabel(c.state.BindMounts["/etc/hosts"], c.config.MountLabel, true); err != nil {
				return err
			}
		}

		if c.state.BindMounts["/etc/resolv.conf"] != "" {
			if err := label.Relabel(c.state.BindMounts["/etc/resolv.conf"], c.config.MountLabel, true); err != nil {
				return err
			}
		}
	} else {
		if !c.config.UseImageHosts && c.state.BindMounts["/etc/hosts"] == "" {
			newHosts, err := c.generateHosts("/etc/hosts")
			if err != nil {
				return errors.Wrapf(err, "error creating hosts file for container %s", c.ID())
			}
			c.state.BindMounts["/etc/hosts"] = newHosts
		}
	}

	// SHM is always added when we mount the container
	c.state.BindMounts["/dev/shm"] = c.config.ShmDir

	newPasswd, newGroup, err := c.generatePasswdAndGroup()
	if err != nil {
		return errors.Wrapf(err, "error creating temporary passwd file for container %s", c.ID())
	}
	if newPasswd != "" {
		// Make /etc/passwd
		// If it already exists, delete so we can recreate
		delete(c.state.BindMounts, "/etc/passwd")
		c.state.BindMounts["/etc/passwd"] = newPasswd
	}
	if newGroup != "" {
		// Make /etc/group
		// If it already exists, delete so we can recreate
		delete(c.state.BindMounts, "/etc/group")
		c.state.BindMounts["/etc/group"] = newGroup
	}

	// Make /etc/hostname
	// This should never change, so no need to recreate if it exists
	if _, ok := c.state.BindMounts["/etc/hostname"]; !ok {
		hostnamePath, err := c.writeStringToRundir("hostname", c.Hostname())
		if err != nil {
			return errors.Wrapf(err, "error creating hostname file for container %s", c.ID())
		}
		c.state.BindMounts["/etc/hostname"] = hostnamePath
	}

	// Make /etc/localtime
	ctrTimezone := c.Timezone()
	if ctrTimezone != "" {
		// validate the format of the timezone specified if it's not "local"
		if ctrTimezone != "local" {
			_, err = time.LoadLocation(ctrTimezone)
			if err != nil {
				return errors.Wrapf(err, "error finding timezone for container %s", c.ID())
			}
		}
		if _, ok := c.state.BindMounts["/etc/localtime"]; !ok {
			var zonePath string
			if ctrTimezone == "local" {
				zonePath, err = filepath.EvalSymlinks("/etc/localtime")
				if err != nil {
					return errors.Wrapf(err, "error finding local timezone for container %s", c.ID())
				}
			} else {
				zone := filepath.Join("/usr/share/zoneinfo", ctrTimezone)
				zonePath, err = filepath.EvalSymlinks(zone)
				if err != nil {
					return errors.Wrapf(err, "error setting timezone for container %s", c.ID())
				}
			}
			localtimePath, err := c.copyTimezoneFile(zonePath)
			if err != nil {
				return errors.Wrapf(err, "error setting timezone for container %s", c.ID())
			}
			c.state.BindMounts["/etc/localtime"] = localtimePath
		}
	}

	// Make .containerenv if it does not exist
	if _, ok := c.state.BindMounts["/run/.containerenv"]; !ok {
		containerenv := c.runtime.graphRootMountedFlag(c.config.Spec.Mounts)
		isRootless := 0
		if rootless.IsRootless() {
			isRootless = 1
		}
		imageID, imageName := c.Image()

		if c.Privileged() {
			// Populate the .containerenv with container information
			containerenv = fmt.Sprintf(`engine="podman-%s"
name=%q
id=%q
image=%q
imageid=%q
rootless=%d
%s`, version.Version.String(), c.Name(), c.ID(), imageName, imageID, isRootless, containerenv)
		}
		containerenvPath, err := c.writeStringToRundir(".containerenv", containerenv)
		if err != nil {
			return errors.Wrapf(err, "error creating containerenv file for container %s", c.ID())
		}
		c.state.BindMounts["/run/.containerenv"] = containerenvPath
	}

	// Add Subscription Mounts
	subscriptionMounts := subscriptions.MountsWithUIDGID(c.config.MountLabel, c.state.RunDir, c.runtime.config.Containers.DefaultMountsFile, c.state.Mountpoint, c.RootUID(), c.RootGID(), rootless.IsRootless(), false)
	for _, mount := range subscriptionMounts {
		if _, ok := c.state.BindMounts[mount.Destination]; !ok {
			c.state.BindMounts[mount.Destination] = mount.Source
		}
	}

	// Secrets are mounted by getting the secret data from the secrets manager,
	// copying the data into the container's static dir,
	// then mounting the copied dir into /run/secrets.
	// The secrets mounting must come after subscription mounts, since subscription mounts
	// creates the /run/secrets dir in the container where we mount as well.
	if len(c.Secrets()) > 0 {
		// create /run/secrets if subscriptions did not create
		if err := c.createSecretMountDir(); err != nil {
			return errors.Wrapf(err, "error creating secrets mount")
		}
		for _, secret := range c.Secrets() {
			secretFileName := secret.Name
			base := "/run/secrets"
			if secret.Target != "" {
				secretFileName = secret.Target
				//If absolute path for target given remove base.
				if filepath.IsAbs(secretFileName) {
					base = ""
				}
			}
			src := filepath.Join(c.config.SecretsPath, secret.Name)
			dest := filepath.Join(base, secretFileName)
			c.state.BindMounts[dest] = src
		}
	}

	return nil
}

// generateResolvConf generates a containers resolv.conf
func (c *Container) generateResolvConf() (string, error) {
	var (
		nameservers      []string
		cniNameServers   []string
		cniSearchDomains []string
	)

	resolvConf := "/etc/resolv.conf"
	for _, namespace := range c.config.Spec.Linux.Namespaces {
		if namespace.Type == spec.NetworkNamespace {
			if namespace.Path != "" && !strings.HasPrefix(namespace.Path, "/proc/") {
				definedPath := filepath.Join("/etc/netns", filepath.Base(namespace.Path), "resolv.conf")
				_, err := os.Stat(definedPath)
				if err == nil {
					resolvConf = definedPath
				} else if !os.IsNotExist(err) {
					return "", err
				}
			}
			break
		}
	}

	contents, err := ioutil.ReadFile(resolvConf)
	// resolv.conf doesn't have to exists
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	ns := resolvconf.GetNameservers(contents)
	// check if systemd-resolved is used, assume it is used when 127.0.0.53 is the only nameserver
	if len(ns) == 1 && ns[0] == "127.0.0.53" {
		// read the actual resolv.conf file for systemd-resolved
		resolvedContents, err := ioutil.ReadFile("/run/systemd/resolve/resolv.conf")
		if err != nil {
			if !os.IsNotExist(err) {
				return "", errors.Wrapf(err, "detected that systemd-resolved is in use, but could not locate real resolv.conf")
			}
		} else {
			contents = resolvedContents
		}
	}

	ipv6 := false
	// Check if CNI gave back and DNS servers for us to add in
	cniResponse := c.state.NetworkStatus
	for _, i := range cniResponse {
		for _, ip := range i.IPs {
			// Note: only using To16() does not work since it also returns a valid ip for ipv4
			if ip.Address.IP.To4() == nil && ip.Address.IP.To16() != nil {
				ipv6 = true
			}
		}
		if i.DNS.Nameservers != nil {
			cniNameServers = append(cniNameServers, i.DNS.Nameservers...)
			logrus.Debugf("adding nameserver(s) from cni response of '%q'", i.DNS.Nameservers)
		}
		if i.DNS.Search != nil {
			cniSearchDomains = append(cniSearchDomains, i.DNS.Search...)
			logrus.Debugf("adding search domain(s) from cni response of '%q'", i.DNS.Search)
		}
	}

	if c.config.NetMode.IsSlirp4netns() {
		ctrNetworkSlipOpts := []string{}
		if c.config.NetworkOptions != nil {
			ctrNetworkSlipOpts = append(ctrNetworkSlipOpts, c.config.NetworkOptions["slirp4netns"]...)
		}
		slirpOpts, err := parseSlirp4netnsNetworkOptions(c.runtime, ctrNetworkSlipOpts)
		if err != nil {
			return "", err
		}
		ipv6 = slirpOpts.enableIPv6
	}

	// Ensure that the container's /etc/resolv.conf is compatible with its
	// network configuration.
	resolv, err := resolvconf.FilterResolvDNS(contents, ipv6, c.config.CreateNetNS)
	if err != nil {
		return "", errors.Wrapf(err, "error parsing host resolv.conf")
	}

	dns := make([]net.IP, 0, len(c.runtime.config.Containers.DNSServers))
	for _, i := range c.runtime.config.Containers.DNSServers {
		result := net.ParseIP(i)
		if result == nil {
			return "", errors.Wrapf(define.ErrInvalidArg, "invalid IP address %s", i)
		}
		dns = append(dns, result)
	}
	dnsServers := append(dns, c.config.DNSServer...)
	// If the user provided dns, it trumps all; then dns masq; then resolv.conf
	switch {
	case len(dnsServers) > 0:

		// We store DNS servers as net.IP, so need to convert to string
		for _, server := range dnsServers {
			nameservers = append(nameservers, server.String())
		}
	case len(cniNameServers) > 0:
		nameservers = append(nameservers, cniNameServers...)
	default:
		// Make a new resolv.conf
		nameservers = resolvconf.GetNameservers(resolv.Content)
		// slirp4netns has a built in DNS server.
		if c.config.NetMode.IsSlirp4netns() {
			slirp4netnsDNS, err := GetSlirp4netnsDNS(c.slirp4netnsSubnet)
			if err != nil {
				logrus.Warn("failed to determine Slirp4netns DNS: ", err.Error())
			} else {
				nameservers = append([]string{slirp4netnsDNS.String()}, nameservers...)
			}
		}
	}

	var search []string
	if len(c.config.DNSSearch) > 0 || len(c.runtime.config.Containers.DNSSearches) > 0 || len(cniSearchDomains) > 0 {
		if !util.StringInSlice(".", c.config.DNSSearch) {
			search = c.runtime.config.Containers.DNSSearches
			search = append(search, c.config.DNSSearch...)
			search = append(search, cniSearchDomains...)
		}
	} else {
		search = resolvconf.GetSearchDomains(resolv.Content)
	}

	var options []string
	if len(c.config.DNSOption) > 0 || len(c.runtime.config.Containers.DNSOptions) > 0 {
		options = c.runtime.config.Containers.DNSOptions
		options = append(options, c.config.DNSOption...)
	} else {
		options = resolvconf.GetOptions(resolv.Content)
	}

	destPath := filepath.Join(c.state.RunDir, "resolv.conf")

	if err := os.Remove(destPath); err != nil && !os.IsNotExist(err) {
		return "", errors.Wrapf(err, "container %s", c.ID())
	}

	// Build resolv.conf
	if _, err = resolvconf.Build(destPath, nameservers, search, options); err != nil {
		return "", errors.Wrapf(err, "error building resolv.conf for container %s", c.ID())
	}

	// Relabel resolv.conf for the container
	if err := label.Relabel(destPath, c.config.MountLabel, true); err != nil {
		return "", err
	}

	return destPath, nil
}

// generateHosts creates a containers hosts file
func (c *Container) generateHosts(path string) (string, error) {
	orig, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	hosts := string(orig)
	hosts += c.getHosts()

	hosts = c.appendLocalhost(hosts)

	return c.writeStringToRundir("hosts", hosts)
}

// based on networking mode we may want to append the localhost
// if there isn't any record for it and also this shoud happen
// in slirp4netns and similar network modes.
func (c *Container) appendLocalhost(hosts string) string {
	if !strings.Contains(hosts, "localhost") &&
		!c.config.NetMode.IsHost() {
		hosts += "127.0.0.1\tlocalhost\n::1\tlocalhost\n"
	}

	return hosts
}

// appendHosts appends a container's config and state pertaining to hosts to a container's
// local hosts file. netCtr is the container from which the netNS information is
// taken.
// path is the basis of the hosts file, into which netCtr's netNS information will be appended.
// FIXME.  Path should be used by this function,but I am not sure what is correct; remove //lint
// once this is fixed
func (c *Container) appendHosts(path string, netCtr *Container) (string, error) { //nolint
	return c.appendStringToRunDir("hosts", netCtr.getHosts())
}

// getHosts finds the pertinent information for a container's host file in its config and state
// and returns a string in a format that can be written to the host file
func (c *Container) getHosts() string {
	var hosts string

	if len(c.config.HostAdd) > 0 {
		for _, host := range c.config.HostAdd {
			// the host format has already been verified at this point
			fields := strings.SplitN(host, ":", 2)
			hosts += fmt.Sprintf("%s %s\n", fields[1], fields[0])
		}
	}

	hosts += c.cniHosts()

	// Add hostname for slirp4netns
	if c.Hostname() != "" {
		if c.config.NetMode.IsSlirp4netns() {
			// When using slirp4netns, the interface gets a static IP
			slirp4netnsIP, err := GetSlirp4netnsIP(c.slirp4netnsSubnet)
			if err != nil {
				logrus.Warnf("failed to determine slirp4netnsIP: %v", err.Error())
			} else {
				hosts += fmt.Sprintf("# used by slirp4netns\n%s\t%s %s\n", slirp4netnsIP.String(), c.Hostname(), c.config.Name)
			}
		}

		// Do we have a network namespace?
		netNone := false
		if c.config.NetNsCtr == "" && !c.config.CreateNetNS {
			for _, ns := range c.config.Spec.Linux.Namespaces {
				if ns.Type == spec.NetworkNamespace {
					if ns.Path == "" {
						netNone = true
					}
					break
				}
			}
		}
		// If we are net=none (have a network namespace, but not connected to
		// anything) add the container's name and hostname to localhost.
		if netNone {
			hosts += fmt.Sprintf("127.0.0.1 %s %s\n", c.Hostname(), c.config.Name)
		}
	}

	// Add gateway entry if we are not in a machine. If we use podman machine
	// the gvproxy dns server will take care of host.containers.internal.
	// https://github.com/containers/gvisor-tap-vsock/commit/1108ea45162281046d239047a6db9bc187e64b08
	if !c.runtime.config.Engine.MachineEnabled {
		var depCtr *Container
		if c.config.NetNsCtr != "" {
			// ignoring the error because there isn't anything to do
			depCtr, _ = c.getRootNetNsDepCtr()
		} else if len(c.state.NetworkStatus) != 0 {
			depCtr = c
		} else {
			depCtr = nil
		}

		if depCtr != nil {
			for _, pluginResultsRaw := range depCtr.state.NetworkStatus {
				pluginResult, _ := cnitypes.GetResult(pluginResultsRaw)
				for _, ip := range pluginResult.IPs {
					hosts += fmt.Sprintf("%s host.containers.internal\n", ip.Gateway)
				}
			}
		} else if c.config.NetMode.IsSlirp4netns() {
			gatewayIP, err := GetSlirp4netnsGateway(c.slirp4netnsSubnet)
			if err != nil {
				logrus.Warn("failed to determine gatewayIP: ", err.Error())
			} else {
				hosts += fmt.Sprintf("%s host.containers.internal\n", gatewayIP.String())
			}
		} else {
			logrus.Debug("network configuration does not support host.containers.internal address")
		}
	}

	return hosts
}

// generateGroupEntry generates an entry or entries into /etc/group as
// required by container configuration.
// Generally speaking, we will make an entry under two circumstances:
// 1. The container is started as a specific user:group, and that group is both
//    numeric, and does not already exist in /etc/group.
// 2. It is requested that Libpod add the group that launched Podman to
//    /etc/group via AddCurrentUserPasswdEntry (though this does not trigger if
//    the group in question already exists in /etc/passwd).
// Returns group entry (as a string that can be appended to /etc/group) and any
// error that occurred.
func (c *Container) generateGroupEntry() (string, error) {
	groupString := ""

	// Things we *can't* handle: adding the user we added in
	// generatePasswdEntry to any *existing* groups.
	addedGID := 0
	if c.config.AddCurrentUserPasswdEntry {
		entry, gid, err := c.generateCurrentUserGroupEntry()
		if err != nil {
			return "", err
		}
		groupString += entry
		addedGID = gid
	}
	if c.config.User != "" {
		entry, _, err := c.generateUserGroupEntry(addedGID)
		if err != nil {
			return "", err
		}
		groupString += entry
	}

	return groupString, nil
}

// Make an entry in /etc/group for the group of the user running podman iff we
// are rootless.
func (c *Container) generateCurrentUserGroupEntry() (string, int, error) {
	gid := rootless.GetRootlessGID()
	if gid == 0 {
		return "", 0, nil
	}

	g, err := user.LookupGroupId(strconv.Itoa(gid))
	if err != nil {
		return "", 0, errors.Wrapf(err, "failed to get current group")
	}

	// Lookup group name to see if it exists in the image.
	_, err = lookup.GetGroup(c.state.Mountpoint, g.Name)
	if err != runcuser.ErrNoGroupEntries {
		return "", 0, err
	}

	// Lookup GID to see if it exists in the image.
	_, err = lookup.GetGroup(c.state.Mountpoint, g.Gid)
	if err != runcuser.ErrNoGroupEntries {
		return "", 0, err
	}

	// We need to get the username of the rootless user so we can add it to
	// the group.
	username := ""
	uid := rootless.GetRootlessUID()
	if uid != 0 {
		u, err := user.LookupId(strconv.Itoa(uid))
		if err != nil {
			return "", 0, errors.Wrapf(err, "failed to get current user to make group entry")
		}
		username = u.Username
	}

	// Make the entry.
	return fmt.Sprintf("%s:x:%s:%s\n", g.Name, g.Gid, username), gid, nil
}

// Make an entry in /etc/group for the group the container was specified to run
// as.
func (c *Container) generateUserGroupEntry(addedGID int) (string, int, error) {
	if c.config.User == "" {
		return "", 0, nil
	}

	splitUser := strings.SplitN(c.config.User, ":", 2)
	group := splitUser[0]
	if len(splitUser) > 1 {
		group = splitUser[1]
	}

	gid, err := strconv.ParseUint(group, 10, 32)
	if err != nil {
		return "", 0, nil
	}

	if addedGID != 0 && addedGID == int(gid) {
		return "", 0, nil
	}

	// Check if the group already exists
	_, err = lookup.GetGroup(c.state.Mountpoint, group)
	if err != runcuser.ErrNoGroupEntries {
		return "", 0, err
	}

	return fmt.Sprintf("%d:x:%d:%s\n", gid, gid, splitUser[0]), int(gid), nil
}

// generatePasswdEntry generates an entry or entries into /etc/passwd as
// required by container configuration.
// Generally speaking, we will make an entry under two circumstances:
// 1. The container is started as a specific user who is not in /etc/passwd.
//    This only triggers if the user is given as a *numeric* ID.
// 2. It is requested that Libpod add the user that launched Podman to
//    /etc/passwd via AddCurrentUserPasswdEntry (though this does not trigger if
//    the user in question already exists in /etc/passwd) or the UID to be added
//    is 0).
// Returns password entry (as a string that can be appended to /etc/passwd) and
// any error that occurred.
func (c *Container) generatePasswdEntry() (string, error) {
	passwdString := ""

	addedUID := 0
	if c.config.AddCurrentUserPasswdEntry {
		entry, uid, _, err := c.generateCurrentUserPasswdEntry()
		if err != nil {
			return "", err
		}
		passwdString += entry
		addedUID = uid
	}
	if c.config.User != "" {
		entry, _, _, err := c.generateUserPasswdEntry(addedUID)
		if err != nil {
			return "", err
		}
		passwdString += entry
	}

	return passwdString, nil
}

// generateCurrentUserPasswdEntry generates an /etc/passwd entry for the user
// running the container engine.
// Returns a passwd entry for the user, and the UID and GID of the added entry.
func (c *Container) generateCurrentUserPasswdEntry() (string, int, int, error) {
	uid := rootless.GetRootlessUID()
	if uid == 0 {
		return "", 0, 0, nil
	}

	u, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		return "", 0, 0, errors.Wrapf(err, "failed to get current user")
	}

	// Lookup the user to see if it exists in the container image.
	_, err = lookup.GetUser(c.state.Mountpoint, u.Username)
	if err != runcuser.ErrNoPasswdEntries {
		return "", 0, 0, err
	}

	// Lookup the UID to see if it exists in the container image.
	_, err = lookup.GetUser(c.state.Mountpoint, u.Uid)
	if err != runcuser.ErrNoPasswdEntries {
		return "", 0, 0, err
	}

	// If the user's actual home directory exists, or was mounted in - use
	// that.
	homeDir := c.WorkingDir()
	hDir := u.HomeDir
	for hDir != "/" {
		if MountExists(c.config.Spec.Mounts, hDir) {
			homeDir = u.HomeDir
			break
		}
		hDir = filepath.Dir(hDir)
	}
	if homeDir != u.HomeDir {
		for _, hDir := range c.UserVolumes() {
			if hDir == u.HomeDir {
				homeDir = u.HomeDir
				break
			}
		}
	}
	// Set HOME environment if not already set
	hasHomeSet := false
	for _, s := range c.config.Spec.Process.Env {
		if strings.HasPrefix(s, "HOME=") {
			hasHomeSet = true
			break
		}
	}
	if !hasHomeSet {
		c.config.Spec.Process.Env = append(c.config.Spec.Process.Env, fmt.Sprintf("HOME=%s", homeDir))
	}

	return fmt.Sprintf("%s:*:%s:%s:%s:%s:/bin/sh\n", u.Username, u.Uid, u.Gid, u.Name, homeDir), uid, rootless.GetRootlessGID(), nil
}

// generateUserPasswdEntry generates an /etc/passwd entry for the container user
// to run in the container.
// The UID and GID of the added entry will also be returned.
// Accepts one argument, that being any UID that has already been added to the
// passwd file by other functions; if it matches the UID we were given, we don't
// need to do anything.
func (c *Container) generateUserPasswdEntry(addedUID int) (string, int, int, error) {
	var (
		groupspec string
		gid       int
	)
	if c.config.User == "" {
		return "", 0, 0, nil
	}
	splitSpec := strings.SplitN(c.config.User, ":", 2)
	userspec := splitSpec[0]
	if len(splitSpec) > 1 {
		groupspec = splitSpec[1]
	}
	// If a non numeric User, then don't generate passwd
	uid, err := strconv.ParseUint(userspec, 10, 32)
	if err != nil {
		return "", 0, 0, nil
	}

	if addedUID != 0 && int(uid) == addedUID {
		return "", 0, 0, nil
	}

	// Lookup the user to see if it exists in the container image
	_, err = lookup.GetUser(c.state.Mountpoint, userspec)
	if err != runcuser.ErrNoPasswdEntries {
		return "", 0, 0, err
	}

	if groupspec != "" {
		ugid, err := strconv.ParseUint(groupspec, 10, 32)
		if err == nil {
			gid = int(ugid)
		} else {
			group, err := lookup.GetGroup(c.state.Mountpoint, groupspec)
			if err != nil {
				return "", 0, 0, errors.Wrapf(err, "unable to get gid %s from group file", groupspec)
			}
			gid = group.Gid
		}
	}
	return fmt.Sprintf("%d:*:%d:%d:container user:%s:/bin/sh\n", uid, uid, gid, c.WorkingDir()), int(uid), gid, nil
}

// generatePasswdAndGroup generates container-specific passwd and group files
// iff g.config.User is a number or we are configured to make a passwd entry for
// the current user.
// Returns path to file to mount at /etc/passwd, path to file to mount at
// /etc/group, and any error that occurred. If no passwd/group file were
// required, the empty string will be returned for those path (this may occur
// even if no error happened).
// This may modify the mounted container's /etc/passwd and /etc/group instead of
// making copies to bind-mount in, so we don't break useradd (it wants to make a
// copy of /etc/passwd and rename the copy to /etc/passwd, which is impossible
// with a bind mount). This is done in cases where the container is *not*
// read-only. In this case, the function will return nothing ("", "", nil).
func (c *Container) generatePasswdAndGroup() (string, string, error) {
	if !c.config.AddCurrentUserPasswdEntry && c.config.User == "" {
		return "", "", nil
	}

	needPasswd := true
	needGroup := true

	// First, check if there's a mount at /etc/passwd or group, we don't
	// want to interfere with user mounts.
	if MountExists(c.config.Spec.Mounts, "/etc/passwd") {
		needPasswd = false
	}
	if MountExists(c.config.Spec.Mounts, "/etc/group") {
		needGroup = false
	}

	// Next, check if we already made the files. If we didn't, don't need to
	// do anything more.
	if needPasswd {
		passwdPath := filepath.Join(c.config.StaticDir, "passwd")
		if _, err := os.Stat(passwdPath); err == nil {
			needPasswd = false
		}
	}
	if needGroup {
		groupPath := filepath.Join(c.config.StaticDir, "group")
		if _, err := os.Stat(groupPath); err == nil {
			needGroup = false
		}
	}

	// Next, check if the container even has a /etc/passwd or /etc/group.
	// If it doesn't we don't want to create them ourselves.
	if needPasswd {
		exists, err := c.checkFileExistsInRootfs("/etc/passwd")
		if err != nil {
			return "", "", err
		}
		needPasswd = exists
	}
	if needGroup {
		exists, err := c.checkFileExistsInRootfs("/etc/group")
		if err != nil {
			return "", "", err
		}
		needGroup = exists
	}

	// If we don't need a /etc/passwd or /etc/group at this point we can
	// just return.
	if !needPasswd && !needGroup {
		return "", "", nil
	}

	passwdPath := ""
	groupPath := ""

	ro := c.IsReadOnly()

	if needPasswd {
		passwdEntry, err := c.generatePasswdEntry()
		if err != nil {
			return "", "", err
		}

		needsWrite := passwdEntry != ""
		switch {
		case ro && needsWrite:
			logrus.Debugf("Making /etc/passwd for container %s", c.ID())
			originPasswdFile, err := securejoin.SecureJoin(c.state.Mountpoint, "/etc/passwd")
			if err != nil {
				return "", "", errors.Wrapf(err, "error creating path to container %s /etc/passwd", c.ID())
			}
			orig, err := ioutil.ReadFile(originPasswdFile)
			if err != nil && !os.IsNotExist(err) {
				return "", "", err
			}
			passwdFile, err := c.writeStringToStaticDir("passwd", string(orig)+passwdEntry)
			if err != nil {
				return "", "", errors.Wrapf(err, "failed to create temporary passwd file")
			}
			if err := os.Chmod(passwdFile, 0644); err != nil {
				return "", "", err
			}
			passwdPath = passwdFile
		case !ro && needsWrite:
			logrus.Debugf("Modifying container %s /etc/passwd", c.ID())
			containerPasswd, err := securejoin.SecureJoin(c.state.Mountpoint, "/etc/passwd")
			if err != nil {
				return "", "", errors.Wrapf(err, "error looking up location of container %s /etc/passwd", c.ID())
			}

			f, err := os.OpenFile(containerPasswd, os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				return "", "", errors.Wrapf(err, "container %s", c.ID())
			}
			defer f.Close()

			if _, err := f.WriteString(passwdEntry); err != nil {
				return "", "", errors.Wrapf(err, "unable to append to container %s /etc/passwd", c.ID())
			}
		default:
			logrus.Debugf("Not modifying container %s /etc/passwd", c.ID())
		}
	}
	if needGroup {
		groupEntry, err := c.generateGroupEntry()
		if err != nil {
			return "", "", err
		}

		needsWrite := groupEntry != ""
		switch {
		case ro && needsWrite:
			logrus.Debugf("Making /etc/group for container %s", c.ID())
			originGroupFile, err := securejoin.SecureJoin(c.state.Mountpoint, "/etc/group")
			if err != nil {
				return "", "", errors.Wrapf(err, "error creating path to container %s /etc/group", c.ID())
			}
			orig, err := ioutil.ReadFile(originGroupFile)
			if err != nil && !os.IsNotExist(err) {
				return "", "", err
			}
			groupFile, err := c.writeStringToStaticDir("group", string(orig)+groupEntry)
			if err != nil {
				return "", "", errors.Wrapf(err, "failed to create temporary group file")
			}
			if err := os.Chmod(groupFile, 0644); err != nil {
				return "", "", err
			}
			groupPath = groupFile
		case !ro && needsWrite:
			logrus.Debugf("Modifying container %s /etc/group", c.ID())
			containerGroup, err := securejoin.SecureJoin(c.state.Mountpoint, "/etc/group")
			if err != nil {
				return "", "", errors.Wrapf(err, "error looking up location of container %s /etc/group", c.ID())
			}

			f, err := os.OpenFile(containerGroup, os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				return "", "", errors.Wrapf(err, "container %s", c.ID())
			}
			defer f.Close()

			if _, err := f.WriteString(groupEntry); err != nil {
				return "", "", errors.Wrapf(err, "unable to append to container %s /etc/group", c.ID())
			}
		default:
			logrus.Debugf("Not modifying container %s /etc/group", c.ID())
		}
	}

	return passwdPath, groupPath, nil
}

func isRootlessCgroupSet(cgroup string) bool {
	// old versions of podman were setting the CgroupParent to CgroupfsDefaultCgroupParent
	// by default.  Avoid breaking these versions and check whether the cgroup parent is
	// set to the default and in this case enable the old behavior.  It should not be a real
	// problem because the default CgroupParent is usually owned by root so rootless users
	// cannot access it.
	// This check might be lifted in a future version of Podman.
	// Check both that the cgroup or its parent is set to the default value (used by pods).
	return cgroup != CgroupfsDefaultCgroupParent && filepath.Dir(cgroup) != CgroupfsDefaultCgroupParent
}

// Get cgroup path in a format suitable for the OCI spec
func (c *Container) getOCICgroupPath() (string, error) {
	unified, err := cgroups.IsCgroup2UnifiedMode()
	if err != nil {
		return "", err
	}
	cgroupManager := c.CgroupManager()
	switch {
	case c.config.NoCgroups:
		return "", nil
	case c.config.CgroupsMode == cgroupSplit:
		selfCgroup, err := utils.GetOwnCgroup()
		if err != nil {
			return "", err
		}
		return filepath.Join(selfCgroup, fmt.Sprintf("libpod-payload-%s", c.ID())), nil
	case cgroupManager == config.SystemdCgroupsManager:
		// When the OCI runtime is set to use Systemd as a cgroup manager, it
		// expects cgroups to be passed as follows:
		// slice:prefix:name
		systemdCgroups := fmt.Sprintf("%s:libpod:%s", path.Base(c.config.CgroupParent), c.ID())
		logrus.Debugf("Setting CGroups for container %s to %s", c.ID(), systemdCgroups)
		return systemdCgroups, nil
	case (rootless.IsRootless() && (cgroupManager == config.CgroupfsCgroupsManager || !unified)):
		if c.config.CgroupParent == "" || !isRootlessCgroupSet(c.config.CgroupParent) {
			return "", nil
		}
		fallthrough
	case cgroupManager == config.CgroupfsCgroupsManager:
		cgroupPath := filepath.Join(c.config.CgroupParent, fmt.Sprintf("libpod-%s", c.ID()))
		logrus.Debugf("Setting CGroup path for container %s to %s", c.ID(), cgroupPath)
		return cgroupPath, nil
	default:
		return "", errors.Wrapf(define.ErrInvalidArg, "invalid cgroup manager %s requested", cgroupManager)
	}
}

func (c *Container) copyTimezoneFile(zonePath string) (string, error) {
	var localtimeCopy string = filepath.Join(c.state.RunDir, "localtime")
	file, err := os.Stat(zonePath)
	if err != nil {
		return "", err
	}
	if file.IsDir() {
		return "", errors.New("Invalid timezone: is a directory")
	}
	src, err := os.Open(zonePath)
	if err != nil {
		return "", err
	}
	defer src.Close()
	dest, err := os.Create(localtimeCopy)
	if err != nil {
		return "", err
	}
	defer dest.Close()
	_, err = io.Copy(dest, src)
	if err != nil {
		return "", err
	}
	if err := label.Relabel(localtimeCopy, c.config.MountLabel, false); err != nil {
		return "", err
	}
	if err := dest.Chown(c.RootUID(), c.RootGID()); err != nil {
		return "", err
	}
	return localtimeCopy, err
}

func (c *Container) cleanupOverlayMounts() error {
	return overlay.CleanupContent(c.config.StaticDir)
}

// Check if a file exists at the given path in the container's root filesystem.
// Container must already be mounted for this to be used.
func (c *Container) checkFileExistsInRootfs(file string) (bool, error) {
	checkPath, err := securejoin.SecureJoin(c.state.Mountpoint, file)
	if err != nil {
		return false, errors.Wrapf(err, "cannot create path to container %s file %q", c.ID(), file)
	}
	stat, err := os.Stat(checkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "container %s", c.ID())
	}
	if stat.IsDir() {
		return false, nil
	}
	return true, nil
}

// Creates and mounts an empty dir to mount secrets into, if it does not already exist
func (c *Container) createSecretMountDir() error {
	src := filepath.Join(c.state.RunDir, "/run/secrets")
	_, err := os.Stat(src)
	if os.IsNotExist(err) {
		oldUmask := umask.Set(0)
		defer umask.Set(oldUmask)

		if err := os.MkdirAll(src, 0755); err != nil {
			return err
		}
		if err := label.Relabel(src, c.config.MountLabel, false); err != nil {
			return err
		}
		if err := os.Chown(src, c.RootUID(), c.RootGID()); err != nil {
			return err
		}
		c.state.BindMounts["/run/secrets"] = src
		return nil
	}

	return err
}

// Fix ownership and permissions of the specified volume if necessary.
func (c *Container) fixVolumePermissions(v *ContainerNamedVolume) error {
	vol, err := c.runtime.state.Volume(v.Name)
	if err != nil {
		return errors.Wrapf(err, "error retrieving named volume %s for container %s", v.Name, c.ID())
	}

	vol.lock.Lock()
	defer vol.lock.Unlock()

	// The volume may need a copy-up. Check the state.
	if err := vol.update(); err != nil {
		return err
	}

	// TODO: For now, I've disabled chowning volumes owned by non-Podman
	// drivers. This may be safe, but it's really going to be a case-by-case
	// thing, I think - safest to leave disabled now and re-enable later if
	// there is a demand.
	if vol.state.NeedsChown && !vol.UsesVolumeDriver() {
		vol.state.NeedsChown = false

		uid := int(c.config.Spec.Process.User.UID)
		gid := int(c.config.Spec.Process.User.GID)

		if c.config.IDMappings.UIDMap != nil {
			p := idtools.IDPair{
				UID: uid,
				GID: gid,
			}
			mappings := idtools.NewIDMappingsFromMaps(c.config.IDMappings.UIDMap, c.config.IDMappings.GIDMap)
			newPair, err := mappings.ToHost(p)
			if err != nil {
				return errors.Wrapf(err, "error mapping user %d:%d", uid, gid)
			}
			uid = newPair.UID
			gid = newPair.GID
		}

		vol.state.UIDChowned = uid
		vol.state.GIDChowned = gid

		if err := vol.save(); err != nil {
			return err
		}

		mountPoint, err := vol.MountPoint()
		if err != nil {
			return err
		}

		if err := os.Lchown(mountPoint, uid, gid); err != nil {
			return err
		}

		// Make sure the new volume matches the permissions of the target directory.
		// https://github.com/containers/podman/issues/10188
		st, err := os.Lstat(filepath.Join(c.state.Mountpoint, v.Dest))
		if err == nil {
			if stat, ok := st.Sys().(*syscall.Stat_t); ok {
				if err := os.Lchown(mountPoint, int(stat.Uid), int(stat.Gid)); err != nil {
					return err
				}
			}
			if err := os.Chmod(mountPoint, st.Mode()); err != nil {
				return err
			}
			stat := st.Sys().(*syscall.Stat_t)
			atime := time.Unix(int64(stat.Atim.Sec), int64(stat.Atim.Nsec))
			if err := os.Chtimes(mountPoint, atime, st.ModTime()); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
