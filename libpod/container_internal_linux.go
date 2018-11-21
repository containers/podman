// +build linux

package libpod

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	cnitypes "github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/plugins/pkg/ns"
	crioAnnotations "github.com/containers/libpod/pkg/annotations"
	"github.com/containers/libpod/pkg/criu"
	"github.com/containers/libpod/pkg/lookup"
	"github.com/containers/libpod/pkg/resolvconf"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/secrets"
	"github.com/containers/storage/pkg/idtools"
	"github.com/opencontainers/runc/libcontainer/user"
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
	if err := unix.Unmount(mount, unix.MNT_DETACH); err != nil {
		if err != syscall.EINVAL {
			logrus.Warnf("container %s failed to unmount %s : %v", c.ID(), mount, err)
		}
	}
	return nil
}

// prepare mounts the container and sets up other required resources like net
// namespaces
func (c *Container) prepare() (err error) {
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
		if c.config.CreateNetNS && c.state.NetNS == nil && !c.config.PostConfigureNetNS {
			netNS, networkStatus, createNetNSErr = c.runtime.createNetNS(c)

			tmpStateLock.Lock()
			defer tmpStateLock.Unlock()

			// Assign NetNS attributes to container
			if createNetNSErr == nil {
				c.state.NetNS = netNS
				c.state.NetworkStatus = networkStatus
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
		if c.state.UserNSRoot == "" {
			c.state.RealMountpoint = c.state.Mountpoint
		} else {
			c.state.RealMountpoint = filepath.Join(c.state.UserNSRoot, "mountpoint")
		}

		logrus.Debugf("Created root filesystem for container %s at %s", c.ID(), c.state.Mountpoint)
	}()

	defer func() {
		if err != nil {
			if err2 := c.cleanupNetwork(); err2 != nil {
				logrus.Errorf("Error cleaning up container %s network: %v", c.ID(), err2)
			}
			if err2 := c.cleanupStorage(); err2 != nil {
				logrus.Errorf("Error cleaning up container %s storage: %v", c.ID(), err2)
			}
		}
	}()

	wg.Wait()

	if createNetNSErr != nil {
		if mountStorageErr != nil {
			logrus.Error(createNetNSErr)
			return mountStorageErr
		}
		return createNetNSErr
	}
	if mountStorageErr != nil {
		return mountStorageErr
	}

	// Save the container
	return c.save()
}

// cleanupNetwork unmounts and cleans up the container's network
func (c *Container) cleanupNetwork() error {
	if c.NetworkDisabled() {
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

// Generate spec for a container
// Accepts a map of the container's dependencies
func (c *Container) generateSpec(ctx context.Context) (*spec.Spec, error) {
	execUser, err := lookup.GetUserGroupInfo(c.state.Mountpoint, c.config.User, nil)
	if err != nil {
		return nil, err
	}
	g := generate.NewFromSpec(c.config.Spec)

	// If network namespace was requested, add it now
	if c.config.CreateNetNS {
		if c.config.PostConfigureNetNS {
			g.AddOrReplaceLinuxNamespace(spec.NetworkNamespace, "")
		} else {
			g.AddOrReplaceLinuxNamespace(spec.NetworkNamespace, c.state.NetNS.Path())
		}
	}

	if err := c.makeBindMounts(); err != nil {
		return nil, err
	}

	// Check if the spec file mounts contain the label Relabel flags z or Z.
	// If they do, relabel the source directory and then remove the option.
	for _, m := range g.Mounts() {
		var options []string
		for _, o := range m.Options {
			switch o {
			case "z":
				fallthrough
			case "Z":
				if err := label.Relabel(m.Source, c.MountLabel(), label.IsShared(o)); err != nil {
					return nil, errors.Wrapf(err, "relabel failed %q", m.Source)
				}

			default:
				options = append(options, o)
			}
		}
		m.Options = options
	}

	g.SetProcessSelinuxLabel(c.ProcessLabel())
	g.SetLinuxMountLabel(c.MountLabel())
	// Remove the default /dev/shm mount to ensure we overwrite it
	g.RemoveMount("/dev/shm")

	// Add bind mounts to container
	for dstPath, srcPath := range c.state.BindMounts {
		newMount := spec.Mount{
			Type:        "bind",
			Source:      srcPath,
			Destination: dstPath,
			Options:     []string{"bind", "private"},
		}
		if c.IsReadOnly() {
			newMount.Options = append(newMount.Options, "ro")
		}
		if !MountExists(g.Mounts(), dstPath) {
			g.AddMount(newMount)
		} else {
			logrus.Warnf("User mount overriding libpod mount at %q", dstPath)
		}
	}

	if !rootless.IsRootless() {
		if c.state.ExtensionStageHooks, err = c.setupOCIHooks(ctx, g.Config); err != nil {
			return nil, errors.Wrapf(err, "error setting up OCI Hooks")
		}
	}

	// Bind builtin image volumes
	if c.config.Rootfs == "" && c.config.ImageVolumes {
		if err := c.addLocalVolumes(ctx, &g); err != nil {
			return nil, errors.Wrapf(err, "error mounting image volumes")
		}
	}

	if c.config.User != "" {
		if !c.state.Mounted {
			return nil, errors.Wrapf(ErrCtrStateInvalid, "container %s must be mounted in order to translate User field", c.ID())
		}
		// User and Group must go together
		g.SetProcessUID(uint32(execUser.Uid))
		g.SetProcessGID(uint32(execUser.Gid))
	}

	// Add addition groups if c.config.GroupAdd is not empty
	if len(c.config.Groups) > 0 {
		if !c.state.Mounted {
			return nil, errors.Wrapf(ErrCtrStateInvalid, "container %s must be mounted in order to add additional groups", c.ID())
		}
		gids, _ := lookup.GetContainerGroups(c.config.Groups, c.state.Mountpoint, nil)
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
	if !rootless.IsRootless() && !strings.Contains(c.config.User, ":") {
		for _, gid := range execUser.Sgids {
			g.AddProcessAdditionalGid(uint32(gid))
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
		if err := c.addNamespaceContainer(&g, PIDNS, c.config.PIDNsCtr, string(spec.PIDNamespace)); err != nil {
			return nil, err
		}
	}
	if c.config.UserNsCtr != "" {
		if err := c.addNamespaceContainer(&g, UserNS, c.config.UserNsCtr, spec.UserNamespace); err != nil {
			return nil, err
		}
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

	if c.config.Rootfs == "" {
		if err := idtools.MkdirAllAs(c.state.RealMountpoint, 0700, c.RootUID(), c.RootGID()); err != nil {
			return nil, err
		}
	}

	g.SetRootPath(c.state.RealMountpoint)
	g.AddAnnotation(crioAnnotations.Created, c.config.CreatedTime.Format(time.RFC3339Nano))
	g.AddAnnotation("org.opencontainers.image.stopSignal", fmt.Sprintf("%d", c.config.StopSignal))

	for _, i := range c.config.Spec.Linux.Namespaces {
		if string(i.Type) == spec.UTSNamespace {
			hostname := c.Hostname()
			g.SetHostname(hostname)
			g.AddProcessEnv("HOSTNAME", hostname)
			break
		}
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

	if rootless.IsRootless() {
		g.SetLinuxCgroupsPath("")
	} else if c.runtime.config.CgroupManager == SystemdCgroupsManager {
		// When runc is set to use Systemd as a cgroup manager, it
		// expects cgroups to be passed as follows:
		// slice:prefix:name
		systemdCgroups := fmt.Sprintf("%s:libpod:%s", path.Base(c.config.CgroupParent), c.ID())
		logrus.Debugf("Setting CGroups for container %s to %s", c.ID(), systemdCgroups)
		g.SetLinuxCgroupsPath(systemdCgroups)
	} else {
		cgroupPath, err := c.CGroupPath()
		if err != nil {
			return nil, err
		}
		logrus.Debugf("Setting CGroup path for container %s to %s", c.ID(), cgroupPath)
		g.SetLinuxCgroupsPath(cgroupPath)
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
		g.SetLinuxRootPropagation(rootPropagation)
	}
	return g.Config, nil
}

// systemd expects to have /run, /run/lock and /tmp on tmpfs
// It also expects to be able to write to /sys/fs/cgroup/systemd and /var/log/journal
func (c *Container) setupSystemd(mounts []spec.Mount, g generate.Generator) error {
	options := []string{"rw", "rprivate", "noexec", "nosuid", "nodev"}
	for _, dest := range []string{"/run", "/run/lock"} {
		if MountExists(mounts, dest) {
			continue
		}
		tmpfsMnt := spec.Mount{
			Destination: dest,
			Type:        "tmpfs",
			Source:      "tmpfs",
			Options:     append(options, "tmpcopyup", "size=65536k"),
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

	// rootless containers have no write access to /sys/fs/cgroup, so don't
	// add any mount into the container.
	if !rootless.IsRootless() {
		cgroupPath, err := c.CGroupPath()
		if err != nil {
			return err
		}
		sourcePath := filepath.Join("/sys/fs/cgroup/systemd", cgroupPath)

		systemdMnt := spec.Mount{
			Destination: "/sys/fs/cgroup/systemd",
			Type:        "bind",
			Source:      sourcePath,
			Options:     []string{"bind", "private"},
		}
		g.AddMount(systemdMnt)
	} else {
		systemdMnt := spec.Mount{
			Destination: "/sys/fs/cgroup/systemd",
			Type:        "bind",
			Source:      "/sys/fs/cgroup/systemd",
			Options:     []string{"bind", "nodev", "noexec", "nosuid"},
		}
		g.AddMount(systemdMnt)
	}

	return nil
}

// Add an existing container's namespace to the spec
func (c *Container) addNamespaceContainer(g *generate.Generator, ns LinuxNS, ctr string, specNS string) error {
	nsCtr, err := c.runtime.state.Container(ctr)
	if err != nil {
		return errors.Wrapf(err, "error retrieving dependency %s of container %s from state", ctr, c.ID())
	}

	// TODO need unlocked version of this for use in pods
	nsPath, err := nsCtr.NamespacePath(ns)
	if err != nil {
		return err
	}

	if err := g.AddOrReplaceLinuxNamespace(specNS, nsPath); err != nil {
		return err
	}

	return nil
}

func (c *Container) checkpoint(ctx context.Context, options ContainerCheckpointOptions) (err error) {

	if !criu.CheckForCriu() {
		return errors.Errorf("checkpointing a container requires at least CRIU %d", criu.MinCriuVersion)
	}

	if c.state.State != ContainerStateRunning {
		return errors.Wrapf(ErrCtrStateInvalid, "%q is not running, cannot checkpoint", c.state.State)
	}
	if err := c.runtime.ociRuntime.checkpointContainer(c, options); err != nil {
		return err
	}

	// Save network.status. This is needed to restore the container with
	// the same IP. Currently limited to one IP address in a container
	// with one interface.
	formatJSON, err := json.MarshalIndent(c.state.NetworkStatus, "", "	")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(filepath.Join(c.bundlePath(), "network.status"), formatJSON, 0644); err != nil {
		return err
	}

	logrus.Debugf("Checkpointed container %s", c.ID())

	if !options.KeepRunning {
		c.state.State = ContainerStateStopped

		// Cleanup Storage and Network
		if err := c.cleanup(ctx); err != nil {
			return err
		}
	}

	if !options.Keep {
		// Remove log file
		os.Remove(filepath.Join(c.bundlePath(), "dump.log"))
		// Remove statistic file
		os.Remove(filepath.Join(c.bundlePath(), "stats-dump"))
	}

	return c.save()
}

func (c *Container) restore(ctx context.Context, keep bool) (err error) {

	if !criu.CheckForCriu() {
		return errors.Errorf("restoring a container requires at least CRIU %d", criu.MinCriuVersion)
	}

	if (c.state.State != ContainerStateConfigured) && (c.state.State != ContainerStateExited) {
		return errors.Wrapf(ErrCtrStateInvalid, "container %s is running or paused, cannot restore", c.ID())
	}

	// Let's try to stat() CRIU's inventory file. If it does not exist, it makes
	// no sense to try a restore. This is a minimal check if a checkpoint exist.
	if _, err := os.Stat(filepath.Join(c.CheckpointPath(), "inventory.img")); os.IsNotExist(err) {
		return errors.Wrapf(err, "A complete checkpoint for this container cannot be found, cannot restore")
	}

	// Read network configuration from checkpoint
	// Currently only one interface with one IP is supported.
	networkStatusFile, err := os.Open(filepath.Join(c.bundlePath(), "network.status"))
	if err == nil {
		// The file with the network.status does exist. Let's restore the
		// container with the same IP address as during checkpointing.
		defer networkStatusFile.Close()
		var networkStatus []*cnitypes.Result
		networkJSON, err := ioutil.ReadAll(networkStatusFile)
		if err != nil {
			return err
		}
		json.Unmarshal(networkJSON, &networkStatus)
		// Take the first IP address
		var IP net.IP
		if len(networkStatus) > 0 {
			if len(networkStatus[0].IPs) > 0 {
				IP = networkStatus[0].IPs[0].Address.IP
			}
		}
		if IP != nil {
			env := fmt.Sprintf("IP=%s", IP)
			// Tell CNI which IP address we want.
			os.Setenv("CNI_ARGS", env)
			logrus.Debugf("Restoring container with %s", env)
		}
	}

	defer func() {
		if err != nil {
			if err2 := c.cleanup(ctx); err2 != nil {
				logrus.Errorf("error cleaning up container %s: %v", c.ID(), err2)
			}
		}
	}()

	if err := c.prepare(); err != nil {
		return err
	}

	// TODO: use existing way to request static IPs, once it is merged in ocicni
	// https://github.com/cri-o/ocicni/pull/23/

	// CNI_ARGS was used to request a certain IP address. Unconditionally remove it.
	os.Unsetenv("CNI_ARGS")

	// Read config
	jsonPath := filepath.Join(c.bundlePath(), "config.json")
	logrus.Debugf("generate.NewFromFile at %v", jsonPath)
	g, err := generate.NewFromFile(jsonPath)
	if err != nil {
		logrus.Debugf("generate.NewFromFile failed with %v", err)
		return err
	}

	// We want to have the same network namespace as before.
	if c.config.CreateNetNS {
		g.AddOrReplaceLinuxNamespace(spec.NetworkNamespace, c.state.NetNS.Path())
	}

	// Save the OCI spec to disk
	if err := c.saveSpec(g.Spec()); err != nil {
		return err
	}

	if err := c.makeBindMounts(); err != nil {
		return err
	}

	// Cleanup for a working restore.
	c.removeConmonFiles()

	if err := c.runtime.ociRuntime.createContainer(c, c.config.CgroupParent, true); err != nil {
		return err
	}

	logrus.Debugf("Restored container %s", c.ID())

	c.state.State = ContainerStateRunning

	if !keep {
		// Delete all checkpoint related files. At this point, in theory, all files
		// should exist. Still ignoring errors for now as the container should be
		// restored and running. Not erroring out just because some cleanup operation
		// failed. Starting with the checkpoint directory
		err = os.RemoveAll(c.CheckpointPath())
		if err != nil {
			logrus.Debugf("Non-fatal: removal of checkpoint directory (%s) failed: %v", c.CheckpointPath(), err)
		}
		cleanup := [...]string{"restore.log", "dump.log", "stats-dump", "stats-restore", "network.status"}
		for _, delete := range cleanup {
			file := filepath.Join(c.bundlePath(), delete)
			err = os.Remove(file)
			if err != nil {
				logrus.Debugf("Non-fatal: removal of checkpoint file (%s) failed: %v", file, err)
			}
		}
	}

	return c.save()
}

// Make standard bind mounts to include in the container
func (c *Container) makeBindMounts() error {
	if err := os.Chown(c.state.RunDir, c.RootUID(), c.RootGID()); err != nil {
		return errors.Wrapf(err, "cannot chown run directory %s", c.state.RunDir)
	}

	if c.state.BindMounts == nil {
		c.state.BindMounts = make(map[string]string)
	}

	if !c.NetworkDisabled() {
		// Make /etc/resolv.conf
		if _, ok := c.state.BindMounts["/etc/resolv.conf"]; ok {
			// If it already exists, delete so we can recreate
			delete(c.state.BindMounts, "/etc/resolv.conf")
		}
		newResolv, err := c.generateResolvConf()
		if err != nil {
			return errors.Wrapf(err, "error creating resolv.conf for container %s", c.ID())
		}
		c.state.BindMounts["/etc/resolv.conf"] = newResolv

		// Make /etc/hosts
		if _, ok := c.state.BindMounts["/etc/hosts"]; ok {
			// If it already exists, delete so we can recreate
			delete(c.state.BindMounts, "/etc/hosts")
		}
		newHosts, err := c.generateHosts()
		if err != nil {
			return errors.Wrapf(err, "error creating hosts file for container %s", c.ID())
		}
		c.state.BindMounts["/etc/hosts"] = newHosts

	}

	// SHM is always added when we mount the container
	c.state.BindMounts["/dev/shm"] = c.config.ShmDir

	newPasswd, err := c.generatePasswd()
	if err != nil {
		return errors.Wrapf(err, "error creating temporary passwd file for container %s", c.ID())
	}
	if newPasswd != "" {
		// Make /etc/passwd
		if _, ok := c.state.BindMounts["/etc/passwd"]; ok {
			// If it already exists, delete so we can recreate
			delete(c.state.BindMounts, "/etc/passwd")
		}
		logrus.Debugf("adding entry to /etc/passwd for non existent default user")
		c.state.BindMounts["/etc/passwd"] = newPasswd
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

	// Make .containerenv
	// Empty file, so no need to recreate if it exists
	if _, ok := c.state.BindMounts["/run/.containerenv"]; !ok {
		// Empty string for now, but we may consider populating this later
		containerenvPath, err := c.writeStringToRundir(".containerenv", "")
		if err != nil {
			return errors.Wrapf(err, "error creating containerenv file for container %s", c.ID())
		}
		c.state.BindMounts["/run/.containerenv"] = containerenvPath
	}

	// Add Secret Mounts
	secretMounts := secrets.SecretMountsWithUIDGID(c.config.MountLabel, c.state.RunDir, c.runtime.config.DefaultMountsFile, c.state.DestinationRunDir, c.RootUID(), c.RootGID())
	for _, mount := range secretMounts {
		if _, ok := c.state.BindMounts[mount.Destination]; !ok {
			c.state.BindMounts[mount.Destination] = mount.Source
		}
	}

	return nil
}

// generateResolvConf generates a containers resolv.conf
func (c *Container) generateResolvConf() (string, error) {
	// Determine the endpoint for resolv.conf in case it is a symlink
	resolvPath, err := filepath.EvalSymlinks("/etc/resolv.conf")
	if err != nil {
		return "", err
	}

	contents, err := ioutil.ReadFile(resolvPath)
	if err != nil {
		return "", errors.Wrapf(err, "unable to read %s", resolvPath)
	}

	// Ensure that the container's /etc/resolv.conf is compatible with its
	// network configuration.
	// TODO: set ipv6 enable bool more sanely
	resolv, err := resolvconf.FilterResolvDNS(contents, true, c.config.CreateNetNS)
	if err != nil {
		return "", errors.Wrapf(err, "error parsing host resolv.conf")
	}

	// Make a new resolv.conf
	nameservers := resolvconf.GetNameservers(resolv.Content)
	if len(c.config.DNSServer) > 0 {
		// We store DNS servers as net.IP, so need to convert to string
		nameservers = []string{}
		for _, server := range c.config.DNSServer {
			nameservers = append(nameservers, server.String())
		}
	}

	search := resolvconf.GetSearchDomains(resolv.Content)
	if len(c.config.DNSSearch) > 0 {
		search = c.config.DNSSearch
	}

	options := resolvconf.GetOptions(resolv.Content)
	if len(c.config.DNSOption) > 0 {
		options = c.config.DNSOption
	}

	destPath := filepath.Join(c.state.RunDir, "resolv.conf")

	if err := os.Remove(destPath); err != nil && !os.IsNotExist(err) {
		return "", errors.Wrapf(err, "error removing resolv.conf for container %s", c.ID())
	}

	// Build resolv.conf
	if _, err = resolvconf.Build(destPath, nameservers, search, options); err != nil {
		return "", errors.Wrapf(err, "error building resolv.conf for container %s")
	}

	// Relabel resolv.conf for the container
	if err := label.Relabel(destPath, c.config.MountLabel, false); err != nil {
		return "", err
	}

	return filepath.Join(c.state.DestinationRunDir, "resolv.conf"), nil
}

// generateHosts creates a containers hosts file
func (c *Container) generateHosts() (string, error) {
	orig, err := ioutil.ReadFile("/etc/hosts")
	if err != nil {
		return "", errors.Wrapf(err, "unable to read /etc/hosts")
	}
	hosts := string(orig)
	if len(c.config.HostAdd) > 0 {
		for _, host := range c.config.HostAdd {
			// the host format has already been verified at this point
			fields := strings.SplitN(host, ":", 2)
			hosts += fmt.Sprintf("%s %s\n", fields[1], fields[0])
		}
	}
	if len(c.state.NetworkStatus) > 0 && len(c.state.NetworkStatus[0].IPs) > 0 {
		ipAddress := strings.Split(c.state.NetworkStatus[0].IPs[0].Address.String(), "/")[0]
		hosts += fmt.Sprintf("%s\t%s\n", ipAddress, c.Hostname())
	}
	return c.writeStringToRundir("hosts", hosts)
}

// generatePasswd generates a container specific passwd file,
// iff g.config.User is a number
func (c *Container) generatePasswd() (string, error) {
	var (
		groupspec string
		group     *user.Group
		gid       int
	)
	if c.config.User == "" {
		return "", nil
	}
	spec := strings.SplitN(c.config.User, ":", 2)
	userspec := spec[0]
	if len(spec) > 1 {
		groupspec = spec[1]
	}
	// If a non numeric User, then don't generate passwd
	uid, err := strconv.ParseUint(userspec, 10, 32)
	if err != nil {
		return "", nil
	}
	// Lookup the user to see if it exists in the container image
	_, err = lookup.GetUser(c.state.Mountpoint, userspec)
	if err != nil && err != user.ErrNoPasswdEntries {
		return "", err
	}
	if err == nil {
		return "", nil
	}
	if groupspec != "" {
		if !c.state.Mounted {
			return "", errors.Wrapf(ErrCtrStateInvalid, "container %s must be mounted in order to translate group field for passwd record", c.ID())
		}
		group, err = lookup.GetGroup(c.state.Mountpoint, groupspec)
		if err != nil {
			if err == user.ErrNoGroupEntries {
				return "", errors.Wrapf(err, "unable to get gid %s from group file", groupspec)
			}
			return "", err
		}
		gid = group.Gid
	}
	originPasswdFile := filepath.Join(c.state.Mountpoint, "/etc/passwd")
	orig, err := ioutil.ReadFile(originPasswdFile)
	if err != nil && !os.IsNotExist(err) {
		return "", errors.Wrapf(err, "unable to read passwd file %s", originPasswdFile)
	}

	pwd := fmt.Sprintf("%s%d:x:%d:%d:container user:%s:/bin/sh\n", orig, uid, uid, gid, c.WorkingDir())
	passwdFile, err := c.writeStringToRundir("passwd", pwd)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create temporary passwd file")
	}
	if os.Chmod(passwdFile, 0644); err != nil {
		return "", err
	}
	return passwdFile, nil
}
