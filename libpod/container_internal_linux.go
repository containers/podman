//go:build linux
// +build linux

package libpod

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containers/buildah/pkg/overlay"
	"github.com/containers/common/libnetwork/etchosts"
	"github.com/containers/common/libnetwork/resolvconf"
	"github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/common/pkg/chown"
	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/subscriptions"
	"github.com/containers/common/pkg/umask"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/lookup"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/containers/podman/v4/utils"
	"github.com/containers/podman/v4/version"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/lockfile"
	securejoin "github.com/cyphar/filepath-securejoin"
	runcuser "github.com/opencontainers/runc/libcontainer/user"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux"
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
			return fmt.Errorf("error unmounting container %s SHM mount %s: %w", c.ID(), mount, err)
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
		netNS                           ns.NetNS
		networkStatus                   map[string]types.StatusBlock
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
			createErr = fmt.Errorf("error unmounting storage for container %s after network create failure: %w", c.ID(), err)
		}
	}

	// It's OK to unconditionally trigger network cleanup. If the network
	// isn't ready it will do nothing.
	if createErr != nil {
		if err := c.cleanupNetwork(); err != nil {
			logrus.Errorf("Preparing container %s: %v", c.ID(), createErr)
			createErr = fmt.Errorf("error cleaning up container %s network after setup failure: %w", c.ID(), err)
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
	if c.state.NetNS == nil {
		logrus.Debugf("Network is already cleaned up, skipping...")
		return nil
	}

	// Stop the container's network namespace (if it has one)
	if err := c.runtime.teardownNetNS(c); err != nil {
		logrus.Errorf("Unable to clean up network for container %s: %q", c.ID(), err)
	}

	c.state.NetNS = nil
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
		return fmt.Errorf("error retrieving dependency %s of container %s from state: %w", ctr, c.ID(), err)
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

// Ensure standard bind mounts are mounted into all root directories (including chroot directories)
func (c *Container) mountIntoRootDirs(mountName string, mountPath string) error {
	c.state.BindMounts[mountName] = mountPath

	for _, chrootDir := range c.config.ChrootDirs {
		c.state.BindMounts[filepath.Join(chrootDir, mountName)] = mountPath
	}

	return nil
}

// Make standard bind mounts to include in the container
func (c *Container) makeBindMounts() error {
	if err := os.Chown(c.state.RunDir, c.RootUID(), c.RootGID()); err != nil {
		return fmt.Errorf("cannot chown run directory: %w", err)
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
					return fmt.Errorf("container %s: %w", c.ID(), err)
				}
				delete(c.state.BindMounts, "/etc/resolv.conf")
			}
			if hostsPath, ok := c.state.BindMounts["/etc/hosts"]; ok {
				if err := os.Remove(hostsPath); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("container %s: %w", c.ID(), err)
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
				return fmt.Errorf("error fetching network namespace dependency container for container %s: %w", c.ID(), err)
			}

			// We need that container's bind mounts
			bindMounts, err := depCtr.BindMounts()
			if err != nil {
				return fmt.Errorf("error fetching bind mounts from dependency %s of container %s: %w", depCtr.ID(), c.ID(), err)
			}

			// The other container may not have a resolv.conf or /etc/hosts
			// If it doesn't, don't copy them
			resolvPath, exists := bindMounts["/etc/resolv.conf"]
			if !c.config.UseImageResolvConf && exists {
				err := c.mountIntoRootDirs("/etc/resolv.conf", resolvPath)

				if err != nil {
					return fmt.Errorf("error assigning mounts to container %s: %w", c.ID(), err)
				}
			}

			// check if dependency container has an /etc/hosts file.
			// It may not have one, so only use it if it does.
			hostsPath, exists := bindMounts[config.DefaultHostsFile]
			if !c.config.UseImageHosts && exists {
				// we cannot use the dependency container lock due ABBA deadlocks in cleanup()
				lock, err := lockfile.GetLockfile(hostsPath)
				if err != nil {
					return fmt.Errorf("failed to lock hosts file: %w", err)
				}
				lock.Lock()

				// add the newly added container to the hosts file
				// we always use 127.0.0.1 as ip since they have the same netns
				err = etchosts.Add(hostsPath, getLocalhostHostEntry(c))
				lock.Unlock()
				if err != nil {
					return fmt.Errorf("error creating hosts file for container %s which depends on container %s: %w", c.ID(), depCtr.ID(), err)
				}

				// finally, save it in the new container
				err = c.mountIntoRootDirs(config.DefaultHostsFile, hostsPath)
				if err != nil {
					return fmt.Errorf("error assigning mounts to container %s: %w", c.ID(), err)
				}
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
				if err := c.generateResolvConf(); err != nil {
					return fmt.Errorf("error creating resolv.conf for container %s: %w", c.ID(), err)
				}
			}

			if !c.config.UseImageHosts {
				if err := c.createHosts(); err != nil {
					return fmt.Errorf("error creating hosts file for container %s: %w", c.ID(), err)
				}
			}
		}

		if c.state.BindMounts["/etc/hosts"] != "" {
			if err := c.relabel(c.state.BindMounts["/etc/hosts"], c.config.MountLabel, true); err != nil {
				return err
			}
		}

		if c.state.BindMounts["/etc/resolv.conf"] != "" {
			if err := c.relabel(c.state.BindMounts["/etc/resolv.conf"], c.config.MountLabel, true); err != nil {
				return err
			}
		}
	} else if !c.config.UseImageHosts && c.state.BindMounts["/etc/hosts"] == "" {
		if err := c.createHosts(); err != nil {
			return fmt.Errorf("error creating hosts file for container %s: %w", c.ID(), err)
		}
	}

	if c.config.ShmDir != "" {
		// If ShmDir has a value SHM is always added when we mount the container
		c.state.BindMounts["/dev/shm"] = c.config.ShmDir
	}

	if c.config.Passwd == nil || *c.config.Passwd {
		newPasswd, newGroup, err := c.generatePasswdAndGroup()
		if err != nil {
			return fmt.Errorf("error creating temporary passwd file for container %s: %w", c.ID(), err)
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
	}

	// Make /etc/hostname
	// This should never change, so no need to recreate if it exists
	if _, ok := c.state.BindMounts["/etc/hostname"]; !ok {
		hostnamePath, err := c.writeStringToRundir("hostname", c.Hostname())
		if err != nil {
			return fmt.Errorf("error creating hostname file for container %s: %w", c.ID(), err)
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
				return fmt.Errorf("error finding timezone for container %s: %w", c.ID(), err)
			}
		}
		if _, ok := c.state.BindMounts["/etc/localtime"]; !ok {
			var zonePath string
			if ctrTimezone == "local" {
				zonePath, err = filepath.EvalSymlinks("/etc/localtime")
				if err != nil {
					return fmt.Errorf("error finding local timezone for container %s: %w", c.ID(), err)
				}
			} else {
				zone := filepath.Join("/usr/share/zoneinfo", ctrTimezone)
				zonePath, err = filepath.EvalSymlinks(zone)
				if err != nil {
					return fmt.Errorf("error setting timezone for container %s: %w", c.ID(), err)
				}
			}
			localtimePath, err := c.copyTimezoneFile(zonePath)
			if err != nil {
				return fmt.Errorf("error setting timezone for container %s: %w", c.ID(), err)
			}
			c.state.BindMounts["/etc/localtime"] = localtimePath
		}
	}

	_, hasRunContainerenv := c.state.BindMounts["/run/.containerenv"]
	if !hasRunContainerenv {
		// check in the spec mounts
		for _, m := range c.config.Spec.Mounts {
			if m.Destination == "/run/.containerenv" || m.Destination == "/run" {
				hasRunContainerenv = true
				break
			}
		}
	}

	// Make .containerenv if it does not exist
	if !hasRunContainerenv {
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
			return fmt.Errorf("error creating containerenv file for container %s: %w", c.ID(), err)
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
			return fmt.Errorf("error creating secrets mount: %w", err)
		}
		for _, secret := range c.Secrets() {
			secretFileName := secret.Name
			base := "/run/secrets"
			if secret.Target != "" {
				secretFileName = secret.Target
				// If absolute path for target given remove base.
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
func (c *Container) generateResolvConf() error {
	var (
		networkNameServers   []string
		networkSearchDomains []string
	)

	netStatus := c.getNetworkStatus()
	for _, status := range netStatus {
		if status.DNSServerIPs != nil {
			for _, nsIP := range status.DNSServerIPs {
				networkNameServers = append(networkNameServers, nsIP.String())
			}
			logrus.Debugf("Adding nameserver(s) from network status of '%q'", status.DNSServerIPs)
		}
		if status.DNSSearchDomains != nil {
			networkSearchDomains = append(networkSearchDomains, status.DNSSearchDomains...)
			logrus.Debugf("Adding search domain(s) from network status of '%q'", status.DNSSearchDomains)
		}
	}

	ipv6, err := c.checkForIPv6(netStatus)
	if err != nil {
		return err
	}

	nameservers := make([]string, 0, len(c.runtime.config.Containers.DNSServers)+len(c.config.DNSServer))
	nameservers = append(nameservers, c.runtime.config.Containers.DNSServers...)
	for _, ip := range c.config.DNSServer {
		nameservers = append(nameservers, ip.String())
	}
	// If the user provided dns, it trumps all; then dns masq; then resolv.conf
	var search []string
	keepHostServers := false
	if len(nameservers) == 0 {
		keepHostServers = true
		// first add the nameservers from the networks status
		nameservers = networkNameServers
		// when we add network dns server we also have to add the search domains
		search = networkSearchDomains
		// slirp4netns has a built in DNS forwarder.
		if c.config.NetMode.IsSlirp4netns() {
			slirp4netnsDNS, err := GetSlirp4netnsDNS(c.slirp4netnsSubnet)
			if err != nil {
				logrus.Warn("Failed to determine Slirp4netns DNS: ", err.Error())
			} else {
				nameservers = append(nameservers, slirp4netnsDNS.String())
			}
		}
	}

	if len(c.config.DNSSearch) > 0 || len(c.runtime.config.Containers.DNSSearches) > 0 {
		customSearch := make([]string, 0, len(c.config.DNSSearch)+len(c.runtime.config.Containers.DNSSearches))
		customSearch = append(customSearch, c.runtime.config.Containers.DNSSearches...)
		customSearch = append(customSearch, c.config.DNSSearch...)
		search = customSearch
	}

	options := make([]string, 0, len(c.config.DNSOption)+len(c.runtime.config.Containers.DNSOptions))
	options = append(options, c.runtime.config.Containers.DNSOptions...)
	options = append(options, c.config.DNSOption...)

	destPath := filepath.Join(c.state.RunDir, "resolv.conf")

	if err := resolvconf.New(&resolvconf.Params{
		IPv6Enabled:     ipv6,
		KeepHostServers: keepHostServers,
		Nameservers:     nameservers,
		Namespaces:      c.config.Spec.Linux.Namespaces,
		Options:         options,
		Path:            destPath,
		Searches:        search,
	}); err != nil {
		return fmt.Errorf("error building resolv.conf for container %s: %w", c.ID(), err)
	}

	return c.bindMountRootFile(destPath, resolvconf.DefaultResolvConf)
}

// Check if a container uses IPv6.
func (c *Container) checkForIPv6(netStatus map[string]types.StatusBlock) (bool, error) {
	for _, status := range netStatus {
		for _, netInt := range status.Interfaces {
			for _, netAddress := range netInt.Subnets {
				// Note: only using To16() does not work since it also returns a valid ip for ipv4
				if netAddress.IPNet.IP.To4() == nil && netAddress.IPNet.IP.To16() != nil {
					return true, nil
				}
			}
		}
	}

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

// Add a new nameserver to the container's resolv.conf, ensuring that it is the
// first nameserver present.
// Usable only with running containers.
func (c *Container) addNameserver(ips []string) error {
	// Take no action if container is not running.
	if !c.ensureState(define.ContainerStateRunning, define.ContainerStateCreated) {
		return nil
	}

	// Do we have a resolv.conf at all?
	path, ok := c.state.BindMounts[resolvconf.DefaultResolvConf]
	if !ok {
		return nil
	}

	if err := resolvconf.Add(path, ips); err != nil {
		return fmt.Errorf("adding new nameserver to container %s resolv.conf: %w", c.ID(), err)
	}

	return nil
}

// Remove an entry from the existing resolv.conf of the container.
// Usable only with running containers.
func (c *Container) removeNameserver(ips []string) error {
	// Take no action if container is not running.
	if !c.ensureState(define.ContainerStateRunning, define.ContainerStateCreated) {
		return nil
	}

	// Do we have a resolv.conf at all?
	path, ok := c.state.BindMounts[resolvconf.DefaultResolvConf]
	if !ok {
		return nil
	}

	if err := resolvconf.Remove(path, ips); err != nil {
		return fmt.Errorf("removing nameservers from container %s resolv.conf: %w", c.ID(), err)
	}

	return nil
}

func getLocalhostHostEntry(c *Container) etchosts.HostEntries {
	return etchosts.HostEntries{{IP: "127.0.0.1", Names: []string{c.Hostname(), c.config.Name}}}
}

// getHostsEntries returns the container ip host entries for the correct netmode
func (c *Container) getHostsEntries() (etchosts.HostEntries, error) {
	var entries etchosts.HostEntries
	names := []string{c.Hostname(), c.config.Name}
	switch {
	case c.config.NetMode.IsBridge():
		entries = etchosts.GetNetworkHostEntries(c.state.NetworkStatus, names...)
	case c.config.NetMode.IsSlirp4netns():
		ip, err := GetSlirp4netnsIP(c.slirp4netnsSubnet)
		if err != nil {
			return nil, err
		}
		entries = etchosts.HostEntries{{IP: ip.String(), Names: names}}
	default:
		// check for net=none
		if !c.config.CreateNetNS {
			for _, ns := range c.config.Spec.Linux.Namespaces {
				if ns.Type == spec.NetworkNamespace {
					if ns.Path == "" {
						entries = etchosts.HostEntries{{IP: "127.0.0.1", Names: names}}
					}
					break
				}
			}
		}
	}
	return entries, nil
}

func (c *Container) createHosts() error {
	var containerIPsEntries etchosts.HostEntries
	var err error
	// if we configure the netns after the container create we should not add
	// the hosts here since we have no information about the actual ips
	// instead we will add them in c.completeNetworkSetup()
	if !c.config.PostConfigureNetNS {
		containerIPsEntries, err = c.getHostsEntries()
		if err != nil {
			return fmt.Errorf("failed to get container ip host entries: %w", err)
		}
	}
	baseHostFile, err := etchosts.GetBaseHostFile(c.runtime.config.Containers.BaseHostsFile, c.state.Mountpoint)
	if err != nil {
		return err
	}

	targetFile := filepath.Join(c.state.RunDir, "hosts")
	err = etchosts.New(&etchosts.Params{
		BaseFile:                 baseHostFile,
		ExtraHosts:               c.config.HostAdd,
		ContainerIPs:             containerIPsEntries,
		HostContainersInternalIP: etchosts.GetHostContainersInternalIP(c.runtime.config, c.state.NetworkStatus, c.runtime.network),
		TargetFile:               targetFile,
	})
	if err != nil {
		return err
	}

	return c.bindMountRootFile(targetFile, config.DefaultHostsFile)
}

// bindMountRootFile will chown and relabel the source file to make it usable in the container.
// It will also add the path to the container bind mount map.
// source is the path on the host, dest is the path in the container.
func (c *Container) bindMountRootFile(source, dest string) error {
	if err := os.Chown(source, c.RootUID(), c.RootGID()); err != nil {
		return err
	}
	if err := label.Relabel(source, c.MountLabel(), false); err != nil {
		return err
	}

	return c.mountIntoRootDirs(dest, source)
}

// generateGroupEntry generates an entry or entries into /etc/group as
// required by container configuration.
// Generally speaking, we will make an entry under two circumstances:
//  1. The container is started as a specific user:group, and that group is both
//     numeric, and does not already exist in /etc/group.
//  2. It is requested that Libpod add the group that launched Podman to
//     /etc/group via AddCurrentUserPasswdEntry (though this does not trigger if
//     the group in question already exists in /etc/passwd).
//
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
		entry, err := c.generateUserGroupEntry(addedGID)
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
		return "", 0, fmt.Errorf("failed to get current group: %w", err)
	}

	// Look up group name to see if it exists in the image.
	_, err = lookup.GetGroup(c.state.Mountpoint, g.Name)
	if err != runcuser.ErrNoGroupEntries {
		return "", 0, err
	}

	// Look up GID to see if it exists in the image.
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
			return "", 0, fmt.Errorf("failed to get current user to make group entry: %w", err)
		}
		username = u.Username
	}

	// Make the entry.
	return fmt.Sprintf("%s:x:%s:%s\n", g.Name, g.Gid, username), gid, nil
}

// Make an entry in /etc/group for the group the container was specified to run
// as.
func (c *Container) generateUserGroupEntry(addedGID int) (string, error) {
	if c.config.User == "" {
		return "", nil
	}

	splitUser := strings.SplitN(c.config.User, ":", 2)
	group := splitUser[0]
	if len(splitUser) > 1 {
		group = splitUser[1]
	}

	gid, err := strconv.ParseUint(group, 10, 32)
	if err != nil {
		return "", nil //nolint: nilerr
	}

	if addedGID != 0 && addedGID == int(gid) {
		return "", nil
	}

	// Check if the group already exists
	_, err = lookup.GetGroup(c.state.Mountpoint, group)
	if err != runcuser.ErrNoGroupEntries {
		return "", err
	}

	return fmt.Sprintf("%d:x:%d:%s\n", gid, gid, splitUser[0]), nil
}

// generatePasswdEntry generates an entry or entries into /etc/passwd as
// required by container configuration.
// Generally speaking, we will make an entry under two circumstances:
//  1. The container is started as a specific user who is not in /etc/passwd.
//     This only triggers if the user is given as a *numeric* ID.
//  2. It is requested that Libpod add the user that launched Podman to
//     /etc/passwd via AddCurrentUserPasswdEntry (though this does not trigger if
//     the user in question already exists in /etc/passwd) or the UID to be added
//     is 0).
//  3. The user specified additional host user accounts to add the the /etc/passwd file
//
// Returns password entry (as a string that can be appended to /etc/passwd) and
// any error that occurred.
func (c *Container) generatePasswdEntry() (string, error) {
	passwdString := ""

	addedUID := 0
	for _, userid := range c.config.HostUsers {
		// Look up User on host
		u, err := util.LookupUser(userid)
		if err != nil {
			return "", err
		}
		entry, err := c.userPasswdEntry(u)
		if err != nil {
			return "", err
		}
		passwdString += entry
	}
	if c.config.AddCurrentUserPasswdEntry {
		entry, uid, _, err := c.generateCurrentUserPasswdEntry()
		if err != nil {
			return "", err
		}
		passwdString += entry
		addedUID = uid
	}
	if c.config.User != "" {
		entry, err := c.generateUserPasswdEntry(addedUID)
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
		return "", 0, 0, fmt.Errorf("failed to get current user: %w", err)
	}
	pwd, err := c.userPasswdEntry(u)
	if err != nil {
		return "", 0, 0, err
	}

	return pwd, uid, rootless.GetRootlessGID(), nil
}

func (c *Container) userPasswdEntry(u *user.User) (string, error) {
	// Look up the user to see if it exists in the container image.
	_, err := lookup.GetUser(c.state.Mountpoint, u.Username)
	if err != runcuser.ErrNoPasswdEntries {
		return "", err
	}

	// Look up the UID to see if it exists in the container image.
	_, err = lookup.GetUser(c.state.Mountpoint, u.Uid)
	if err != runcuser.ErrNoPasswdEntries {
		return "", err
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
	if c.config.PasswdEntry != "" {
		return c.passwdEntry(u.Username, u.Uid, u.Gid, u.Name, homeDir), nil
	}

	return fmt.Sprintf("%s:*:%s:%s:%s:%s:/bin/sh\n", u.Username, u.Uid, u.Gid, u.Name, homeDir), nil
}

// generateUserPasswdEntry generates an /etc/passwd entry for the container user
// to run in the container.
// The UID and GID of the added entry will also be returned.
// Accepts one argument, that being any UID that has already been added to the
// passwd file by other functions; if it matches the UID we were given, we don't
// need to do anything.
func (c *Container) generateUserPasswdEntry(addedUID int) (string, error) {
	var (
		groupspec string
		gid       int
	)
	if c.config.User == "" {
		return "", nil
	}
	splitSpec := strings.SplitN(c.config.User, ":", 2)
	userspec := splitSpec[0]
	if len(splitSpec) > 1 {
		groupspec = splitSpec[1]
	}
	// If a non numeric User, then don't generate passwd
	uid, err := strconv.ParseUint(userspec, 10, 32)
	if err != nil {
		return "", nil //nolint: nilerr
	}

	if addedUID != 0 && int(uid) == addedUID {
		return "", nil
	}

	// Look up the user to see if it exists in the container image
	_, err = lookup.GetUser(c.state.Mountpoint, userspec)
	if err != runcuser.ErrNoPasswdEntries {
		return "", err
	}

	if groupspec != "" {
		ugid, err := strconv.ParseUint(groupspec, 10, 32)
		if err == nil {
			gid = int(ugid)
		} else {
			group, err := lookup.GetGroup(c.state.Mountpoint, groupspec)
			if err != nil {
				return "", fmt.Errorf("unable to get gid %s from group file: %w", groupspec, err)
			}
			gid = group.Gid
		}
	}

	if c.config.PasswdEntry != "" {
		entry := c.passwdEntry(fmt.Sprintf("%d", uid), fmt.Sprintf("%d", uid), fmt.Sprintf("%d", gid), "container user", c.WorkingDir())
		return entry, nil
	}

	return fmt.Sprintf("%d:*:%d:%d:container user:%s:/bin/sh\n", uid, uid, gid, c.WorkingDir()), nil
}

func (c *Container) passwdEntry(username string, uid, gid, name, homeDir string) string {
	s := c.config.PasswdEntry
	s = strings.ReplaceAll(s, "$USERNAME", username)
	s = strings.ReplaceAll(s, "$UID", uid)
	s = strings.ReplaceAll(s, "$GID", gid)
	s = strings.ReplaceAll(s, "$NAME", name)
	s = strings.ReplaceAll(s, "$HOME", homeDir)
	return s + "\n"
}

// generatePasswdAndGroup generates container-specific passwd and group files
// iff g.config.User is a number or we are configured to make a passwd entry for
// the current user or the user specified HostsUsers
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
	if !c.config.AddCurrentUserPasswdEntry && c.config.User == "" &&
		len(c.config.HostUsers) == 0 {
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
				return "", "", fmt.Errorf("error creating path to container %s /etc/passwd: %w", c.ID(), err)
			}
			orig, err := ioutil.ReadFile(originPasswdFile)
			if err != nil && !os.IsNotExist(err) {
				return "", "", err
			}
			passwdFile, err := c.writeStringToStaticDir("passwd", string(orig)+passwdEntry)
			if err != nil {
				return "", "", fmt.Errorf("failed to create temporary passwd file: %w", err)
			}
			if err := os.Chmod(passwdFile, 0644); err != nil {
				return "", "", err
			}
			passwdPath = passwdFile
		case !ro && needsWrite:
			logrus.Debugf("Modifying container %s /etc/passwd", c.ID())
			containerPasswd, err := securejoin.SecureJoin(c.state.Mountpoint, "/etc/passwd")
			if err != nil {
				return "", "", fmt.Errorf("error looking up location of container %s /etc/passwd: %w", c.ID(), err)
			}

			f, err := os.OpenFile(containerPasswd, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				return "", "", fmt.Errorf("container %s: %w", c.ID(), err)
			}
			defer f.Close()

			if _, err := f.WriteString(passwdEntry); err != nil {
				return "", "", fmt.Errorf("unable to append to container %s /etc/passwd: %w", c.ID(), err)
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
				return "", "", fmt.Errorf("error creating path to container %s /etc/group: %w", c.ID(), err)
			}
			orig, err := ioutil.ReadFile(originGroupFile)
			if err != nil && !os.IsNotExist(err) {
				return "", "", err
			}
			groupFile, err := c.writeStringToStaticDir("group", string(orig)+groupEntry)
			if err != nil {
				return "", "", fmt.Errorf("failed to create temporary group file: %w", err)
			}
			if err := os.Chmod(groupFile, 0644); err != nil {
				return "", "", err
			}
			groupPath = groupFile
		case !ro && needsWrite:
			logrus.Debugf("Modifying container %s /etc/group", c.ID())
			containerGroup, err := securejoin.SecureJoin(c.state.Mountpoint, "/etc/group")
			if err != nil {
				return "", "", fmt.Errorf("error looking up location of container %s /etc/group: %w", c.ID(), err)
			}

			f, err := os.OpenFile(containerGroup, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				return "", "", fmt.Errorf("container %s: %w", c.ID(), err)
			}
			defer f.Close()

			if _, err := f.WriteString(groupEntry); err != nil {
				return "", "", fmt.Errorf("unable to append to container %s /etc/group: %w", c.ID(), err)
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

func (c *Container) copyTimezoneFile(zonePath string) (string, error) {
	localtimeCopy := filepath.Join(c.state.RunDir, "localtime")
	file, err := os.Stat(zonePath)
	if err != nil {
		return "", err
	}
	if file.IsDir() {
		return "", errors.New("invalid timezone: is a directory")
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
	if err := c.relabel(localtimeCopy, c.config.MountLabel, false); err != nil {
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
		return fmt.Errorf("error retrieving named volume %s for container %s: %w", v.Name, c.ID(), err)
	}

	vol.lock.Lock()
	defer vol.lock.Unlock()

	// The volume may need a copy-up. Check the state.
	if err := vol.update(); err != nil {
		return err
	}

	// Volumes owned by a volume driver are not chowned - we don't want to
	// mess with a mount not managed by us.
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
				return fmt.Errorf("error mapping user %d:%d: %w", uid, gid, err)
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
			atime := time.Unix(int64(stat.Atim.Sec), int64(stat.Atim.Nsec)) //nolint: unconvert
			if err := os.Chtimes(mountPoint, atime, st.ModTime()); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (c *Container) relabel(src, mountLabel string, recurse bool) error {
	if !selinux.GetEnabled() || mountLabel == "" {
		return nil
	}
	// only relabel on initial creation of container
	if !c.ensureState(define.ContainerStateConfigured, define.ContainerStateUnknown) {
		label, err := label.FileLabel(src)
		if err != nil {
			return err
		}
		// If labels are different, might be on a tmpfs
		if label == mountLabel {
			return nil
		}
	}
	return label.Relabel(src, mountLabel, recurse)
}

func (c *Container) ChangeHostPathOwnership(src string, recurse bool, uid, gid int) error {
	// only chown on initial creation of container
	if !c.ensureState(define.ContainerStateConfigured, define.ContainerStateUnknown) {
		st, err := os.Stat(src)
		if err != nil {
			return err
		}

		// If labels are different, might be on a tmpfs
		if int(st.Sys().(*syscall.Stat_t).Uid) == uid && int(st.Sys().(*syscall.Stat_t).Gid) == gid {
			return nil
		}
	}
	return chown.ChangeHostPathOwnership(src, recurse, uid, gid)
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
		err := c.runtime.setupRootlessPortMappingViaRLK(c, c.state.NetNS.Path(), c.state.NetworkStatus)
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
			if err := g.AddOrReplaceLinuxNamespace(string(spec.NetworkNamespace), c.state.NetNS.Path()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Container) addSystemdMounts(g *generate.Generator) error {
	if c.Systemd() {
		if err := c.setupSystemd(g.Mounts(), *g); err != nil {
			return fmt.Errorf("error adding systemd-specific mounts: %w", err)
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
