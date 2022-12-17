//go:build linux
// +build linux

package libpod

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/utils"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

var (
	bindOptions = []string{"bind", "rprivate"}
)

func (c *Container) mountSHM(shmOptions string) error {
	if err := unix.Mount("shm", c.config.ShmDir, "tmpfs", unix.MS_NOEXEC|unix.MS_NOSUID|unix.MS_NODEV,
		label.FormatMountLabel(shmOptions, c.config.MountLabel)); err != nil {
		return fmt.Errorf("failed to mount shm tmpfs %q: %w", c.config.ShmDir, err)
	}
	return nil
}

func (c *Container) unmountSHM(mount string) error {
	if err := unix.Unmount(mount, 0); err != nil {
		if err != syscall.EINVAL && err != syscall.ENOENT {
			return fmt.Errorf("unmounting container %s SHM mount %s: %w", c.ID(), mount, err)
		}
		// If it's just an EINVAL or ENOENT, debug logs only
		logrus.Debugf("Container %s failed to unmount %s : %v", c.ID(), mount, err)
	}
	return nil
}

// prepare mounts the container and sets up other required resources like net
// namespaces
func (c *Container) prepare() error {
	var (
		wg                              sync.WaitGroup
		netNS                           string
		networkStatus                   map[string]types.StatusBlock
		createNetNSErr, mountStorageErr error
		mountPoint                      string
		tmpStateLock                    sync.Mutex
	)

	wg.Add(2)

	go func() {
		defer wg.Done()
		// Set up network namespace if not already set up
		noNetNS := c.state.NetNS == ""
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
			logrus.Errorf("Preparing container %s: %v", c.ID(), createErr)
		}
		createErr = mountStorageErr
	}

	// Only trigger storage cleanup if mountStorage was successful.
	// Otherwise, we may mess up mount counters.
	if createNetNSErr != nil && mountStorageErr == nil {
		if err := c.cleanupStorage(); err != nil {
			// createErr is guaranteed non-nil, so print
			// unconditionally
			logrus.Errorf("Preparing container %s: %v", c.ID(), createErr)
			createErr = fmt.Errorf("unmounting storage for container %s after network create failure: %w", c.ID(), err)
		}
	}

	// It's OK to unconditionally trigger network cleanup. If the network
	// isn't ready it will do nothing.
	if createErr != nil {
		if err := c.cleanupNetwork(); err != nil {
			logrus.Errorf("Preparing container %s: %v", c.ID(), createErr)
			createErr = fmt.Errorf("cleaning up container %s network after setup failure: %w", c.ID(), err)
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
	if c.state.NetNS == "" {
		logrus.Debugf("Network is already cleaned up, skipping...")
		return nil
	}

	// Stop the container's network namespace (if it has one)
	if err := c.runtime.teardownNetNS(c); err != nil {
		logrus.Errorf("Unable to clean up network for container %s: %q", c.ID(), err)
	}

	c.state.NetNS = ""
	c.state.NetworkStatus = nil
	c.state.NetworkStatusOld = nil

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

// systemd expects to have /run, /run/lock and /tmp on tmpfs
// It also expects to be able to write to /sys/fs/cgroup/systemd and /var/log/journal
func (c *Container) setupSystemd(mounts []spec.Mount, g generate.Generator) error {
	var containerUUIDSet bool
	for _, s := range c.config.Spec.Process.Env {
		if strings.HasPrefix(s, "container_uuid=") {
			containerUUIDSet = true
			break
		}
	}
	if !containerUUIDSet {
		g.AddProcessEnv("container_uuid", c.ID()[:32])
	}
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
		skipMount := false

		var statfs unix.Statfs_t
		if err := unix.Statfs("/sys/fs/cgroup/systemd", &statfs); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// If the mount is missing on the host, we cannot bind mount it so
				// just skip it.
				skipMount = true
			}
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
		if !skipMount {
			systemdMnt := spec.Mount{
				Destination: "/sys/fs/cgroup/systemd",
				Type:        "bind",
				Source:      "/sys/fs/cgroup/systemd",
				Options:     mountOptions,
			}
			g.AddMount(systemdMnt)
			g.AddLinuxMaskedPaths("/sys/fs/cgroup/systemd/release_agent")
		}
	}

	return nil
}

// Add an existing container's namespace to the spec
func (c *Container) addNamespaceContainer(g *generate.Generator, ns LinuxNS, ctr string, specNS spec.LinuxNamespaceType) error {
	nsCtr, err := c.runtime.state.Container(ctr)
	if err != nil {
		return fmt.Errorf("retrieving dependency %s of container %s from state: %w", ctr, c.ID(), err)
	}

	if specNS == spec.UTSNamespace {
		hostname := nsCtr.Hostname()
		// Joining an existing namespace, cannot set the hostname
		g.SetHostname("")
		g.AddProcessEnv("HOSTNAME", hostname)
	}

	nsPath, err := nsCtr.NamespacePath(ns)
	if err != nil {
		return err
	}

	if err := g.AddOrReplaceLinuxNamespace(string(specNS), nsPath); err != nil {
		return err
	}

	return nil
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

func (c *Container) expectPodCgroup() (bool, error) {
	unified, err := cgroups.IsCgroup2UnifiedMode()
	if err != nil {
		return false, err
	}
	cgroupManager := c.CgroupManager()
	switch {
	case c.config.NoCgroups:
		return false, nil
	case cgroupManager == config.SystemdCgroupsManager:
		return !rootless.IsRootless() || unified, nil
	case cgroupManager == config.CgroupfsCgroupsManager:
		return !rootless.IsRootless(), nil
	default:
		return false, fmt.Errorf("invalid cgroup mode %s requested for pods: %w", cgroupManager, define.ErrInvalidArg)
	}
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
		selfCgroup, err := utils.GetOwnCgroupDisallowRoot()
		if err != nil {
			return "", err
		}
		return filepath.Join(selfCgroup, fmt.Sprintf("libpod-payload-%s", c.ID())), nil
	case cgroupManager == config.SystemdCgroupsManager:
		// When the OCI runtime is set to use Systemd as a cgroup manager, it
		// expects cgroups to be passed as follows:
		// slice:prefix:name
		systemdCgroups := fmt.Sprintf("%s:libpod:%s", path.Base(c.config.CgroupParent), c.ID())
		logrus.Debugf("Setting Cgroups for container %s to %s", c.ID(), systemdCgroups)
		return systemdCgroups, nil
	case (rootless.IsRootless() && (cgroupManager == config.CgroupfsCgroupsManager || !unified)):
		if c.config.CgroupParent == "" || !isRootlessCgroupSet(c.config.CgroupParent) {
			return "", nil
		}
		fallthrough
	case cgroupManager == config.CgroupfsCgroupsManager:
		cgroupPath := filepath.Join(c.config.CgroupParent, fmt.Sprintf("libpod-%s", c.ID()))
		logrus.Debugf("Setting Cgroup path for container %s to %s", c.ID(), cgroupPath)
		return cgroupPath, nil
	default:
		return "", fmt.Errorf("invalid cgroup manager %s requested: %w", cgroupManager, define.ErrInvalidArg)
	}
}

// If the container is rootless, set up the slirp4netns network
func (c *Container) setupRootlessNetwork() error {
	// set up slirp4netns again because slirp4netns will die when conmon exits
	if c.config.NetMode.IsSlirp4netns() {
		err := c.runtime.setupSlirp4netns(c, c.state.NetNS)
		if err != nil {
			return err
		}
	}

	// set up rootlesskit port forwarder again since it dies when conmon exits
	// we use rootlesskit port forwarder only as rootless and when bridge network is used
	if rootless.IsRootless() && c.config.NetMode.IsBridge() && len(c.config.PortMappings) > 0 {
		err := c.runtime.setupRootlessPortMappingViaRLK(c, c.state.NetNS, c.state.NetworkStatus)
		if err != nil {
			return err
		}
	}
	return nil
}

func openDirectory(path string) (fd int, err error) {
	return unix.Open(path, unix.O_RDONLY|unix.O_PATH, 0)
}

func (c *Container) addNetworkNamespace(g *generate.Generator) error {
	if c.config.CreateNetNS {
		if c.config.PostConfigureNetNS {
			if err := g.AddOrReplaceLinuxNamespace(string(spec.NetworkNamespace), ""); err != nil {
				return err
			}
		} else {
			if err := g.AddOrReplaceLinuxNamespace(string(spec.NetworkNamespace), c.state.NetNS); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Container) addSystemdMounts(g *generate.Generator) error {
	if c.Systemd() {
		if err := c.setupSystemd(g.Mounts(), *g); err != nil {
			return fmt.Errorf("adding systemd-specific mounts: %w", err)
		}
	}
	return nil
}

func (c *Container) addSharedNamespaces(g *generate.Generator) error {
	if c.config.IPCNsCtr != "" {
		if err := c.addNamespaceContainer(g, IPCNS, c.config.IPCNsCtr, spec.IPCNamespace); err != nil {
			return err
		}
	}
	if c.config.MountNsCtr != "" {
		if err := c.addNamespaceContainer(g, MountNS, c.config.MountNsCtr, spec.MountNamespace); err != nil {
			return err
		}
	}
	if c.config.NetNsCtr != "" {
		if err := c.addNamespaceContainer(g, NetNS, c.config.NetNsCtr, spec.NetworkNamespace); err != nil {
			return err
		}
	}
	if c.config.PIDNsCtr != "" {
		if err := c.addNamespaceContainer(g, PIDNS, c.config.PIDNsCtr, spec.PIDNamespace); err != nil {
			return err
		}
	}
	if c.config.UserNsCtr != "" {
		if err := c.addNamespaceContainer(g, UserNS, c.config.UserNsCtr, spec.UserNamespace); err != nil {
			return err
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
			logrus.Debugf("User or group ID mappings not available: %s", err)
		} else {
			return err
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
			return err
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
		if err := c.addNamespaceContainer(g, UTSNS, c.config.UTSNsCtr, spec.UTSNamespace); err != nil {
			return err
		}
	}
	if c.config.CgroupNsCtr != "" {
		if err := c.addNamespaceContainer(g, CgroupNS, c.config.CgroupNsCtr, spec.CgroupNamespace); err != nil {
			return err
		}
	}

	if c.config.UserNsCtr == "" && c.config.IDMappings.AutoUserNs {
		if err := g.AddOrReplaceLinuxNamespace(string(spec.UserNamespace), ""); err != nil {
			return err
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
	return nil
}

func (c *Container) addRootPropagation(g *generate.Generator, mounts []spec.Mount) error {
	// Determine property of RootPropagation based on volume properties. If
	// a volume is shared, then keep root propagation shared. This should
	// work for slave and private volumes too.
	//
	// For slave volumes, it can be either [r]shared/[r]slave.
	//
	// For private volumes any root propagation value should work.
	rootPropagation := ""
	for _, m := range mounts {
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
		logrus.Debugf("Set root propagation to %q", rootPropagation)
		if err := g.SetLinuxRootPropagation(rootPropagation); err != nil {
			return err
		}
	}
	return nil
}

func (c *Container) setProcessLabel(g *generate.Generator) {
	g.SetProcessSelinuxLabel(c.ProcessLabel())
}

func (c *Container) setMountLabel(g *generate.Generator) {
	g.SetLinuxMountLabel(c.MountLabel())
}

func (c *Container) setCgroupsPath(g *generate.Generator) error {
	cgroupPath, err := c.getOCICgroupPath()
	if err != nil {
		return err
	}
	g.SetLinuxCgroupsPath(cgroupPath)
	return nil
}

func (c *Container) addSlirp4netnsDNS(nameservers []string) []string {
	// slirp4netns has a built in DNS forwarder.
	if c.config.NetMode.IsSlirp4netns() {
		slirp4netnsDNS, err := GetSlirp4netnsDNS(c.slirp4netnsSubnet)
		if err != nil {
			logrus.Warn("Failed to determine Slirp4netns DNS: ", err.Error())
		} else {
			nameservers = append(nameservers, slirp4netnsDNS.String())
		}
	}
	return nameservers
}

func (c *Container) isSlirp4netnsIPv6() (bool, error) {
	if c.config.NetMode.IsSlirp4netns() {
		ctrNetworkSlipOpts := []string{}
		if c.config.NetworkOptions != nil {
			ctrNetworkSlipOpts = append(ctrNetworkSlipOpts, c.config.NetworkOptions["slirp4netns"]...)
		}
		slirpOpts, err := parseSlirp4netnsNetworkOptions(c.runtime, ctrNetworkSlipOpts)
		if err != nil {
			return false, err
		}
		return slirpOpts.enableIPv6, nil
	}

	return false, nil
}

// check for net=none
func (c *Container) hasNetNone() bool {
	if !c.config.CreateNetNS {
		for _, ns := range c.config.Spec.Linux.Namespaces {
			if ns.Type == spec.NetworkNamespace {
				if ns.Path == "" {
					return true
				}
			}
		}
	}
	return false
}

func setVolumeAtime(mountPoint string, st os.FileInfo) error {
	stat := st.Sys().(*syscall.Stat_t)
	atime := time.Unix(int64(stat.Atim.Sec), int64(stat.Atim.Nsec)) //nolint: unconvert
	if err := os.Chtimes(mountPoint, atime, st.ModTime()); err != nil {
		return err
	}
	return nil
}

func (c *Container) makePlatformBindMounts() error {
	// Make /etc/hostname
	// This should never change, so no need to recreate if it exists
	if _, ok := c.state.BindMounts["/etc/hostname"]; !ok {
		hostnamePath, err := c.writeStringToRundir("hostname", c.Hostname())
		if err != nil {
			return fmt.Errorf("creating hostname file for container %s: %w", c.ID(), err)
		}
		c.state.BindMounts["/etc/hostname"] = hostnamePath
	}
	return nil
}

func (c *Container) getConmonPidFd() int {
	if c.state.ConmonPID != 0 {
		// Track lifetime of conmon precisely using pidfd_open + poll.
		// There are many cases for this to fail, for instance conmon is dead
		// or pidfd_open is not supported (pre linux 5.3), so fall back to the
		// traditional loop with poll + sleep
		if fd, err := unix.PidfdOpen(c.state.ConmonPID, 0); err == nil {
			return fd
		} else if err != unix.ENOSYS && err != unix.ESRCH {
			logrus.Debugf("PidfdOpen(%d) failed: %v", c.state.ConmonPID, err)
		}
	}
	return -1
}
