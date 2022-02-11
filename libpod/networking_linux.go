// +build linux

package libpod

import (
	"crypto/rand"
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/netns"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/libpod/events"
	"github.com/containers/podman/v4/pkg/errorhandling"
	"github.com/containers/podman/v4/pkg/namespaces"
	"github.com/containers/podman/v4/pkg/resolvconf"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/containers/podman/v4/utils"
	"github.com/containers/storage/pkg/lockfile"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

const (
	// slirp4netnsMTU the default MTU override
	slirp4netnsMTU = 65520

	// default slirp4ns subnet
	defaultSlirp4netnsSubnet = "10.0.2.0/24"

	// rootlessNetNsName is the file name for the rootless network namespace bind mount
	rootlessNetNsName = "rootless-netns"

	// rootlessNetNsSilrp4netnsPidFile is the name of the rootless netns slirp4netns pid file
	rootlessNetNsSilrp4netnsPidFile = "rootless-netns-slirp4netns.pid"

	// persistentCNIDir is the directory where the CNI files are stored
	persistentCNIDir = "/var/lib/cni"
)

// convertPortMappings will remove the HostIP part from the ports when running inside podman machine.
// This is need because a HostIP of 127.0.0.1 would now allow the gvproxy forwarder to reach to open ports.
// For machine the HostIP must only be used by gvproxy and never in the VM.
func (c *Container) convertPortMappings() []types.PortMapping {
	if !c.runtime.config.Engine.MachineEnabled || len(c.config.PortMappings) == 0 {
		return c.config.PortMappings
	}
	// if we run in a machine VM we have to ignore the host IP part
	newPorts := make([]types.PortMapping, 0, len(c.config.PortMappings))
	for _, port := range c.config.PortMappings {
		port.HostIP = ""
		newPorts = append(newPorts, port)
	}
	return newPorts
}

func (c *Container) getNetworkOptions(networkOpts map[string]types.PerNetworkOptions) (types.NetworkOptions, error) {
	opts := types.NetworkOptions{
		ContainerID:   c.config.ID,
		ContainerName: getCNIPodName(c),
	}
	opts.PortMappings = c.convertPortMappings()

	// If the container requested special network options use this instead of the config.
	// This is the case for container restore or network reload.
	if c.perNetworkOpts != nil {
		opts.Networks = c.perNetworkOpts
	} else {
		opts.Networks = networkOpts
	}
	return opts, nil
}

type RootlessNetNS struct {
	ns   ns.NetNS
	dir  string
	Lock lockfile.Locker
}

// getPath will join the given path to the rootless netns dir
func (r *RootlessNetNS) getPath(path string) string {
	return filepath.Join(r.dir, path)
}

// Do - run the given function in the rootless netns.
// It does not lock the rootlessCNI lock, the caller
// should only lock when needed, e.g. for cni operations.
func (r *RootlessNetNS) Do(toRun func() error) error {
	err := r.ns.Do(func(_ ns.NetNS) error {
		// Before we can run the given function,
		// we have to setup all mounts correctly.

		// The order of the mounts is IMPORTANT.
		// The idea of the extra mount ns is to make /run and /var/lib/cni writeable
		// for the cni plugins but not affecting the podman user namespace.
		// Because the plugins also need access to XDG_RUNTIME_DIR/netns some special setup is needed.

		// The following bind mounts are needed
		// 1. XDG_RUNTIME_DIR -> XDG_RUNTIME_DIR/rootless-netns/XDG_RUNTIME_DIR
		// 2. /run/systemd -> XDG_RUNTIME_DIR/rootless-netns/run/systemd (only if it exists)
		// 3. XDG_RUNTIME_DIR/rootless-netns/resolv.conf -> /etc/resolv.conf or XDG_RUNTIME_DIR/rootless-netns/run/symlink/target
		// 4. XDG_RUNTIME_DIR/rootless-netns/var/lib/cni -> /var/lib/cni (if /var/lib/cni does not exists use the parent dir)
		// 5. XDG_RUNTIME_DIR/rootless-netns/run -> /run

		// Create a new mount namespace,
		// this must happen inside the netns thread.
		err := unix.Unshare(unix.CLONE_NEWNS)
		if err != nil {
			return errors.Wrapf(err, "cannot create a new mount namespace")
		}

		xdgRuntimeDir, err := util.GetRuntimeDir()
		if err != nil {
			return errors.Wrap(err, "could not get runtime directory")
		}
		newXDGRuntimeDir := r.getPath(xdgRuntimeDir)
		// 1. Mount the netns into the new run to keep them accessible.
		// Otherwise cni setup will fail because it cannot access the netns files.
		err = unix.Mount(xdgRuntimeDir, newXDGRuntimeDir, "none", unix.MS_BIND|unix.MS_SHARED|unix.MS_REC, "")
		if err != nil {
			return errors.Wrap(err, "failed to mount runtime directory for rootless netns")
		}

		// 2. Also keep /run/systemd if it exists.
		// Many files are symlinked into this dir, for example /dev/log.
		runSystemd := "/run/systemd"
		_, err = os.Stat(runSystemd)
		if err == nil {
			newRunSystemd := r.getPath(runSystemd)
			err = unix.Mount(runSystemd, newRunSystemd, "none", unix.MS_BIND|unix.MS_REC, "")
			if err != nil {
				return errors.Wrap(err, "failed to mount /run/systemd directory for rootless netns")
			}
		}

		// 3. On some distros /etc/resolv.conf is symlinked to somewhere under /run.
		// Because the kernel will follow the symlink before mounting, it is not
		// possible to mount a file at /etc/resolv.conf. We have to ensure that
		// the link target will be available in the mount ns.
		// see: https://github.com/containers/podman/issues/10855
		resolvePath := "/etc/resolv.conf"
		linkCount := 0
		for i := 1; i < len(resolvePath); i++ {
			// Do not use filepath.EvalSymlinks, we only want the first symlink under /run.
			// If /etc/resolv.conf has more than one symlink under /run, e.g.
			// -> /run/systemd/resolve/stub-resolv.conf -> /run/systemd/resolve/resolv.conf
			// we would put the netns resolv.conf file to the last path. However this will
			// break dns because the second link does not exists in the mount ns.
			// see https://github.com/containers/podman/issues/11222
			//
			// We also need to resolve all path components not just the last file.
			// see https://github.com/containers/podman/issues/12461

			if resolvePath[i] != '/' {
				// if we are at the last char we need to inc i by one because there is no final slash
				if i == len(resolvePath)-1 {
					i++
				} else {
					// not the end of path, keep going
					continue
				}
			}
			path := resolvePath[:i]

			fi, err := os.Lstat(path)
			if err != nil {
				return errors.Wrap(err, "failed to stat resolv.conf path")
			}

			// no link, just continue
			if fi.Mode()&os.ModeSymlink == 0 {
				continue
			}

			link, err := os.Readlink(path)
			if err != nil {
				return errors.Wrap(err, "failed to read resolv.conf symlink")
			}
			linkCount++
			if filepath.IsAbs(link) {
				// link is as an absolute path
				resolvePath = filepath.Join(link, resolvePath[i:])
			} else {
				// link is as a relative, join it with the previous path
				base := filepath.Dir(path)
				resolvePath = filepath.Join(base, link, resolvePath[i:])
			}
			// set i back to zero since we now have a new base path
			i = 0

			// we have to stop at the first path under /run because we will have an empty /run and will create the path anyway
			// if we would continue we would need to recreate all links under /run
			if strings.HasPrefix(resolvePath, "/run/") {
				break
			}
			// make sure wo do not loop forever
			if linkCount == 255 {
				return errors.New("too many symlinks while resolving /etc/resolv.conf")
			}
		}
		logrus.Debugf("The path of /etc/resolv.conf in the mount ns is %q", resolvePath)
		// When /etc/resolv.conf on the host is a symlink to /run/systemd/resolve/stub-resolv.conf,
		// we have to mount an empty filesystem on /run/systemd/resolve in the child namespace,
		// so as to isolate the directory from the host mount namespace.
		//
		// Otherwise our bind-mount for /run/systemd/resolve/stub-resolv.conf is unmounted
		// when systemd-resolved unlinks and recreates /run/systemd/resolve/stub-resolv.conf on the host.
		// see: https://github.com/containers/podman/issues/10929
		if strings.HasPrefix(resolvePath, "/run/systemd/resolve/") {
			rsr := r.getPath("/run/systemd/resolve")
			err = unix.Mount("", rsr, "tmpfs", unix.MS_NOEXEC|unix.MS_NOSUID|unix.MS_NODEV, "")
			if err != nil {
				return errors.Wrapf(err, "failed to mount tmpfs on %q for rootless netns", rsr)
			}
		}
		if strings.HasPrefix(resolvePath, "/run/") {
			resolvePath = r.getPath(resolvePath)
			err = os.MkdirAll(filepath.Dir(resolvePath), 0700)
			if err != nil {
				return errors.Wrap(err, "failed to create rootless-netns resolv.conf directory")
			}
			// we want to bind mount on this file so we have to create the file first
			_, err = os.OpenFile(resolvePath, os.O_CREATE|os.O_RDONLY, 0700)
			if err != nil {
				return errors.Wrap(err, "failed to create rootless-netns resolv.conf file")
			}
		}
		// mount resolv.conf to make use of the host dns
		err = unix.Mount(r.getPath("resolv.conf"), resolvePath, "none", unix.MS_BIND, "")
		if err != nil {
			return errors.Wrap(err, "failed to mount resolv.conf for rootless netns")
		}

		// 4. CNI plugins need access to /var/lib/cni and /run
		varDir := ""
		varTarget := persistentCNIDir
		// we can only mount to a target dir which exists, check /var/lib/cni recursively
		// while we could always use /var there are cases where a user might store the cni
		// configs under /var/custom and this would break
		for {
			if _, err := os.Stat(varTarget); err == nil {
				varDir = r.getPath(varTarget)
				break
			}
			varTarget = filepath.Dir(varTarget)
			if varTarget == "/" {
				break
			}
		}
		if varDir == "" {
			return errors.New("failed to stat /var directory")
		}
		// make sure to mount var first
		err = unix.Mount(varDir, varTarget, "none", unix.MS_BIND, "")
		if err != nil {
			return errors.Wrapf(err, "failed to mount %s for rootless netns", varTarget)
		}

		// 5. Mount the new prepared run dir to /run, it has to be recursive to keep the other bind mounts.
		runDir := r.getPath("run")
		err = unix.Mount(runDir, "/run", "none", unix.MS_BIND|unix.MS_REC, "")
		if err != nil {
			return errors.Wrap(err, "failed to mount /run for rootless netns")
		}

		// run the given function in the correct namespace
		err = toRun()
		return err
	})
	return err
}

// Cleanup the rootless network namespace if needed.
// It checks if we have running containers with the bridge network mode.
// Cleanup() expects that r.Lock is locked
func (r *RootlessNetNS) Cleanup(runtime *Runtime) error {
	_, err := os.Stat(r.dir)
	if os.IsNotExist(err) {
		// the directory does not exists no need for cleanup
		return nil
	}
	activeNetns := func(c *Container) bool {
		// no bridge => no need to check
		if !c.config.NetMode.IsBridge() {
			return false
		}

		// we cannot use c.state() because it will try to lock the container
		// locking is a problem because cleanup is called after net teardown
		// at this stage the container is already locked.
		// also do not try to lock only containers which are not currently in net
		// teardown because this will result in an ABBA deadlock between the rootless
		// cni lock and the container lock
		// because we need to get the state we have to sync otherwise this will not
		// work because the state is empty by default
		// I do not like this but I do not see a better way at moment
		err := c.syncContainer()
		if err != nil {
			return false
		}

		// only check for an active netns, we cannot use the container state
		// because not running does not mean that the netns does not need cleanup
		// only if the netns is empty we know that we do not need cleanup
		return c.state.NetNS != nil
	}
	ctrs, err := runtime.GetContainersWithoutLock(activeNetns)
	if err != nil {
		return err
	}
	// no cleanup if we found no other containers with a netns
	// we will always find one container (the container cleanup that is currently calling us)
	if len(ctrs) > 1 {
		return nil
	}
	logrus.Debug("Cleaning up rootless network namespace")
	err = netns.UnmountNS(r.ns)
	if err != nil {
		return err
	}
	// make the following errors not fatal
	err = r.ns.Close()
	if err != nil {
		logrus.Error(err)
	}
	b, err := ioutil.ReadFile(r.getPath(rootlessNetNsSilrp4netnsPidFile))
	if err == nil {
		var i int
		i, err = strconv.Atoi(string(b))
		if err == nil {
			// kill the slirp process so we do not leak it
			err = syscall.Kill(i, syscall.SIGTERM)
		}
	}
	if err != nil {
		logrus.Errorf("Failed to kill slirp4netns process: %s", err)
	}
	err = os.RemoveAll(r.dir)
	if err != nil {
		logrus.Error(err)
	}
	return nil
}

// GetRootlessNetNs returns the rootless netns object. If create is set to true
// the rootless network namespace will be created if it does not exists already.
// If called as root it returns always nil.
// On success the returned RootlessCNI lock is locked and must be unlocked by the caller.
func (r *Runtime) GetRootlessNetNs(new bool) (*RootlessNetNS, error) {
	if !rootless.IsRootless() {
		return nil, nil
	}
	var rootlessNetNS *RootlessNetNS
	runDir := r.config.Engine.TmpDir

	lfile := filepath.Join(runDir, "rootless-netns.lock")
	lock, err := lockfile.GetLockfile(lfile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get rootless-netns lockfile")
	}
	lock.Lock()
	defer func() {
		// In case of an error (early exit) rootlessNetNS will be nil.
		// Make sure to unlock otherwise we could deadlock.
		if rootlessNetNS == nil {
			lock.Unlock()
		}
	}()

	rootlessNetNsDir := filepath.Join(runDir, rootlessNetNsName)
	err = os.MkdirAll(rootlessNetNsDir, 0700)
	if err != nil {
		return nil, errors.Wrap(err, "could not create rootless-netns directory")
	}

	nsDir, err := netns.GetNSRunDir()
	if err != nil {
		return nil, err
	}

	// create a hash from the static dir
	// the cleanup will check if there are running containers
	// if you run a several libpod instances with different root/runroot directories this check will fail
	// we want one netns for each libpod static dir so we use the hash to prevent name collisions
	hash := sha1.Sum([]byte(r.config.Engine.StaticDir))
	netnsName := fmt.Sprintf("%s-%x", rootlessNetNsName, hash[:10])

	path := filepath.Join(nsDir, netnsName)
	ns, err := ns.GetNS(path)
	if err != nil {
		if !new {
			// return a error if we could not get the namespace and should no create one
			return nil, errors.Wrap(err, "error getting rootless network namespace")
		}
		// create a new namespace
		logrus.Debugf("creating rootless network namespace with name %q", netnsName)
		ns, err = netns.NewNSWithName(netnsName)
		if err != nil {
			return nil, errors.Wrap(err, "error creating rootless network namespace")
		}
		// setup slirp4netns here
		path := r.config.Engine.NetworkCmdPath
		if path == "" {
			var err error
			path, err = exec.LookPath("slirp4netns")
			if err != nil {
				return nil, err
			}
		}

		syncR, syncW, err := os.Pipe()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to open pipe")
		}
		defer errorhandling.CloseQuiet(syncR)
		defer errorhandling.CloseQuiet(syncW)

		netOptions, err := parseSlirp4netnsNetworkOptions(r, nil)
		if err != nil {
			return nil, err
		}
		slirpFeatures, err := checkSlirpFlags(path)
		if err != nil {
			return nil, errors.Wrapf(err, "error checking slirp4netns binary %s: %q", path, err)
		}
		cmdArgs, err := createBasicSlirp4netnsCmdArgs(netOptions, slirpFeatures)
		if err != nil {
			return nil, err
		}
		// Note we do not use --exit-fd, we kill this process by pid
		cmdArgs = append(cmdArgs, "-c", "-r", "3")
		cmdArgs = append(cmdArgs, "--netns-type=path", ns.Path(), "tap0")

		cmd := exec.Command(path, cmdArgs...)
		logrus.Debugf("slirp4netns command: %s", strings.Join(cmd.Args, " "))
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}

		// workaround for https://github.com/rootless-containers/slirp4netns/pull/153
		if !netOptions.noPivotRoot && slirpFeatures.HasEnableSandbox {
			cmd.SysProcAttr.Cloneflags = syscall.CLONE_NEWNS
			cmd.SysProcAttr.Unshareflags = syscall.CLONE_NEWNS
		}

		// Leak one end of the pipe in slirp4netns
		cmd.ExtraFiles = append(cmd.ExtraFiles, syncW)

		logPath := filepath.Join(r.config.Engine.TmpDir, "slirp4netns-rootless-netns.log")
		logFile, err := os.Create(logPath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to open slirp4netns log file %s", logPath)
		}
		defer logFile.Close()
		// Unlink immediately the file so we won't need to worry about cleaning it up later.
		// It is still accessible through the open fd logFile.
		if err := os.Remove(logPath); err != nil {
			return nil, errors.Wrapf(err, "delete file %s", logPath)
		}
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		if err := cmd.Start(); err != nil {
			return nil, errors.Wrapf(err, "failed to start slirp4netns process")
		}
		// create pid file for the slirp4netns process
		// this is need to kill the process in the cleanup
		pid := strconv.Itoa(cmd.Process.Pid)
		err = ioutil.WriteFile(filepath.Join(rootlessNetNsDir, rootlessNetNsSilrp4netnsPidFile), []byte(pid), 0700)
		if err != nil {
			errors.Wrap(err, "unable to write rootless-netns slirp4netns pid file")
		}

		defer func() {
			if err := cmd.Process.Release(); err != nil {
				logrus.Errorf("Unable to release command process: %q", err)
			}
		}()

		if err := waitForSync(syncR, cmd, logFile, 1*time.Second); err != nil {
			return nil, err
		}

		// move to systemd scope to prevent systemd from killing it
		err = utils.MoveRootlessNetnsSlirpProcessToUserSlice(cmd.Process.Pid)
		if err != nil {
			logrus.Errorf("failed to move the rootless netns slirp4netns process to the systemd user.slice: %v", err)
		}

		// build a new resolv.conf file which uses the slirp4netns dns server address
		resolveIP, err := GetSlirp4netnsDNS(nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to determine default slirp4netns DNS address")
		}

		if netOptions.cidr != "" {
			_, cidr, err := net.ParseCIDR(netOptions.cidr)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse slirp4netns cidr")
			}
			resolveIP, err = GetSlirp4netnsDNS(cidr)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to determine slirp4netns DNS address from cidr: %s", cidr.String())
			}
		}
		conf, err := resolvconf.Get()
		if err != nil {
			return nil, err
		}
		conf, err = resolvconf.FilterResolvDNS(conf.Content, netOptions.enableIPv6, true)
		if err != nil {
			return nil, err
		}
		searchDomains := resolvconf.GetSearchDomains(conf.Content)
		dnsOptions := resolvconf.GetOptions(conf.Content)
		nameServers := resolvconf.GetNameservers(conf.Content)

		_, err = resolvconf.Build(filepath.Join(rootlessNetNsDir, "resolv.conf"), append([]string{resolveIP.String()}, nameServers...), searchDomains, dnsOptions)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create rootless netns resolv.conf")
		}

		// create cni directories to store files
		// they will be bind mounted to the correct location in a extra mount ns
		err = os.MkdirAll(filepath.Join(rootlessNetNsDir, persistentCNIDir), 0700)
		if err != nil {
			return nil, errors.Wrap(err, "could not create rootless-netns var directory")
		}
		runDir := filepath.Join(rootlessNetNsDir, "run")
		err = os.MkdirAll(runDir, 0700)
		if err != nil {
			return nil, errors.Wrap(err, "could not create rootless-netns run directory")
		}
		// relabel the new run directory to the iptables /run label
		// this is important, otherwise the iptables command will fail
		err = label.Relabel(runDir, "system_u:object_r:iptables_var_run_t:s0", false)
		if err != nil {
			return nil, errors.Wrap(err, "could not create relabel rootless-netns run directory")
		}
		// create systemd run directory
		err = os.MkdirAll(filepath.Join(runDir, "systemd"), 0700)
		if err != nil {
			return nil, errors.Wrap(err, "could not create rootless-netns systemd directory")
		}
		// create the directory for the netns files at the same location
		// relative to the rootless-netns location
		err = os.MkdirAll(filepath.Join(rootlessNetNsDir, nsDir), 0700)
		if err != nil {
			return nil, errors.Wrap(err, "could not create rootless-netns netns directory")
		}
	}

	// The CNI plugins and netavark need access to iptables in $PATH. As it turns out debian doesn't put
	// /usr/sbin in $PATH for rootless users. This will break rootless networking completely.
	// We might break existing users and we cannot expect everyone to change their $PATH so
	// lets add /usr/sbin to $PATH ourselves.
	path = os.Getenv("PATH")
	if !strings.Contains(path, "/usr/sbin") {
		path = path + ":/usr/sbin"
		os.Setenv("PATH", path)
	}

	// Important set rootlessNetNS as last step.
	// Do not return any errors after this.
	rootlessNetNS = &RootlessNetNS{
		ns:   ns,
		dir:  rootlessNetNsDir,
		Lock: lock,
	}
	return rootlessNetNS, nil
}

// setUpNetwork will set up the the networks, on error it will also tear down the cni
// networks. If rootless it will join/create the rootless network namespace.
func (r *Runtime) setUpNetwork(ns string, opts types.NetworkOptions) (map[string]types.StatusBlock, error) {
	rootlessNetNS, err := r.GetRootlessNetNs(true)
	if err != nil {
		return nil, err
	}
	var results map[string]types.StatusBlock
	setUpPod := func() error {
		results, err = r.network.Setup(ns, types.SetupOptions{NetworkOptions: opts})
		return err
	}
	// rootlessNetNS is nil if we are root
	if rootlessNetNS != nil {
		// execute the setup in the rootless net ns
		err = rootlessNetNS.Do(setUpPod)
		rootlessNetNS.Lock.Unlock()
	} else {
		err = setUpPod()
	}
	return results, err
}

// getCNIPodName return the pod name (hostname) used by CNI and the dnsname plugin.
// If we are in the pod network namespace use the pod name otherwise the container name
func getCNIPodName(c *Container) string {
	if c.config.NetMode.IsPod() || c.IsInfra() {
		pod, err := c.runtime.state.Pod(c.PodID())
		if err == nil {
			return pod.Name()
		}
	}
	return c.Name()
}

// Create and configure a new network namespace for a container
func (r *Runtime) configureNetNS(ctr *Container, ctrNS ns.NetNS) (status map[string]types.StatusBlock, rerr error) {
	if err := r.exposeMachinePorts(ctr.config.PortMappings); err != nil {
		return nil, err
	}
	defer func() {
		// make sure to unexpose the gvproxy ports when an error happens
		if rerr != nil {
			if err := r.unexposeMachinePorts(ctr.config.PortMappings); err != nil {
				logrus.Errorf("failed to free gvproxy machine ports: %v", err)
			}
		}
	}()
	if ctr.config.NetMode.IsSlirp4netns() {
		return nil, r.setupSlirp4netns(ctr, ctrNS)
	}
	networks, err := ctr.networks()
	if err != nil {
		return nil, err
	}
	// All networks have been removed from the container.
	// This is effectively forcing net=none.
	if len(networks) == 0 {
		return nil, nil
	}

	netOpts, err := ctr.getNetworkOptions(networks)
	if err != nil {
		return nil, err
	}
	netStatus, err := r.setUpNetwork(ctrNS.Path(), netOpts)
	if err != nil {
		return nil, err
	}

	// setup rootless port forwarder when rootless with ports and the network status is empty,
	// if this is called from network reload the network status will not be empty and we should
	// not setup port because they are still active
	if rootless.IsRootless() && len(ctr.config.PortMappings) > 0 && ctr.getNetworkStatus() == nil {
		// set up port forwarder for rootless netns
		netnsPath := ctrNS.Path()
		// TODO: support slirp4netns port forwarder as well
		// make sure to fix this in container.handleRestartPolicy() as well
		// Important we have to call this after r.setUpNetwork() so that
		// we can use the proper netStatus
		err = r.setupRootlessPortMappingViaRLK(ctr, netnsPath, netStatus)
	}
	return netStatus, err
}

// Create and configure a new network namespace for a container
func (r *Runtime) createNetNS(ctr *Container) (n ns.NetNS, q map[string]types.StatusBlock, retErr error) {
	ctrNS, err := netns.NewNS()
	if err != nil {
		return nil, nil, errors.Wrapf(err, "error creating network namespace for container %s", ctr.ID())
	}
	defer func() {
		if retErr != nil {
			if err := netns.UnmountNS(ctrNS); err != nil {
				logrus.Errorf("Unmounting partially created network namespace for container %s: %v", ctr.ID(), err)
			}
			if err := ctrNS.Close(); err != nil {
				logrus.Errorf("Closing partially created network namespace for container %s: %v", ctr.ID(), err)
			}
		}
	}()

	logrus.Debugf("Made network namespace at %s for container %s", ctrNS.Path(), ctr.ID())

	var networkStatus map[string]types.StatusBlock
	networkStatus, err = r.configureNetNS(ctr, ctrNS)
	return ctrNS, networkStatus, err
}

// Configure the network namespace using the container process
func (r *Runtime) setupNetNS(ctr *Container) error {
	nsProcess := fmt.Sprintf("/proc/%d/ns/net", ctr.state.PID)

	b := make([]byte, 16)

	if _, err := rand.Reader.Read(b); err != nil {
		return errors.Wrapf(err, "failed to generate random netns name")
	}
	nsPath, err := netns.GetNSRunDir()
	if err != nil {
		return err
	}
	nsPath = filepath.Join(nsPath, fmt.Sprintf("netns-%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]))

	if err := os.MkdirAll(filepath.Dir(nsPath), 0711); err != nil {
		return err
	}

	mountPointFd, err := os.Create(nsPath)
	if err != nil {
		return err
	}
	if err := mountPointFd.Close(); err != nil {
		return err
	}

	if err := unix.Mount(nsProcess, nsPath, "none", unix.MS_BIND, ""); err != nil {
		return errors.Wrapf(err, "cannot mount %s", nsPath)
	}

	netNS, err := ns.GetNS(nsPath)
	if err != nil {
		return err
	}
	networkStatus, err := r.configureNetNS(ctr, netNS)

	// Assign NetNS attributes to container
	ctr.state.NetNS = netNS
	ctr.state.NetworkStatus = networkStatus
	return err
}

// Join an existing network namespace
func joinNetNS(path string) (ns.NetNS, error) {
	netNS, err := ns.GetNS(path)
	if err != nil {
		return nil, errors.Wrapf(err, "error retrieving network namespace at %s", path)
	}

	return netNS, nil
}

// Close a network namespace.
// Differs from teardownNetNS() in that it will not attempt to undo the setup of
// the namespace, but will instead only close the open file descriptor
func (r *Runtime) closeNetNS(ctr *Container) error {
	if ctr.state.NetNS == nil {
		// The container has no network namespace, we're set
		return nil
	}

	if err := ctr.state.NetNS.Close(); err != nil {
		return errors.Wrapf(err, "error closing network namespace for container %s", ctr.ID())
	}

	ctr.state.NetNS = nil

	return nil
}

// Tear down a container's network configuration and joins the
// rootless net ns as rootless user
func (r *Runtime) teardownNetwork(ns string, opts types.NetworkOptions) error {
	rootlessNetNS, err := r.GetRootlessNetNs(false)
	if err != nil {
		return err
	}
	tearDownPod := func() error {
		err := r.network.Teardown(ns, types.TeardownOptions{NetworkOptions: opts})
		return errors.Wrapf(err, "error tearing down network namespace configuration for container %s", opts.ContainerID)
	}

	// rootlessNetNS is nil if we are root
	if rootlessNetNS != nil {
		// execute the cni setup in the rootless net ns
		err = rootlessNetNS.Do(tearDownPod)
		if cerr := rootlessNetNS.Cleanup(r); cerr != nil {
			logrus.WithError(err).Error("failed to cleanup rootless netns")
		}
		rootlessNetNS.Lock.Unlock()
	} else {
		err = tearDownPod()
	}
	return err
}

// Tear down a container's CNI network configuration, but do not tear down the
// namespace itself.
func (r *Runtime) teardownCNI(ctr *Container) error {
	if ctr.state.NetNS == nil {
		// The container has no network namespace, we're set
		return nil
	}

	logrus.Debugf("Tearing down network namespace at %s for container %s", ctr.state.NetNS.Path(), ctr.ID())

	networks, err := ctr.networks()
	if err != nil {
		return err
	}

	if !ctr.config.NetMode.IsSlirp4netns() && len(networks) > 0 {
		netOpts, err := ctr.getNetworkOptions(networks)
		if err != nil {
			return err
		}
		return r.teardownNetwork(ctr.state.NetNS.Path(), netOpts)
	}
	return nil
}

// Tear down a network namespace, undoing all state associated with it.
func (r *Runtime) teardownNetNS(ctr *Container) error {
	if err := r.unexposeMachinePorts(ctr.config.PortMappings); err != nil {
		// do not return an error otherwise we would prevent network cleanup
		logrus.Errorf("failed to free gvproxy machine ports: %v", err)
	}
	if err := r.teardownCNI(ctr); err != nil {
		return err
	}

	// First unmount the namespace
	if err := netns.UnmountNS(ctr.state.NetNS); err != nil {
		return errors.Wrapf(err, "error unmounting network namespace for container %s", ctr.ID())
	}

	// Now close the open file descriptor
	if err := ctr.state.NetNS.Close(); err != nil {
		return errors.Wrapf(err, "error closing network namespace for container %s", ctr.ID())
	}

	ctr.state.NetNS = nil

	return nil
}

func getContainerNetNS(ctr *Container) (string, *Container, error) {
	if ctr.state.NetNS != nil {
		return ctr.state.NetNS.Path(), nil, nil
	}
	if ctr.config.NetNsCtr != "" {
		c, err := ctr.runtime.GetContainer(ctr.config.NetNsCtr)
		if err != nil {
			return "", nil, err
		}
		if err = c.syncContainer(); err != nil {
			return "", c, err
		}
		netNs, c2, err := getContainerNetNS(c)
		if c2 != nil {
			c = c2
		}
		return netNs, c, err
	}
	return "", nil, nil
}

// isBridgeNetMode checks if the given network mode is bridge.
// It returns nil when it is set to bridge and an error otherwise.
func isBridgeNetMode(n namespaces.NetworkMode) error {
	if !n.IsBridge() {
		return errors.Wrapf(define.ErrNetworkModeInvalid, "%q is not supported", n)
	}
	return nil
}

// Reload only works with containers with a configured network.
// It will tear down, and then reconfigure, the network of the container.
// This is mainly used when a reload of firewall rules wipes out existing
// firewall configuration.
// Efforts will be made to preserve MAC and IP addresses, but this only works if
// the container only joined a single CNI network, and was only assigned a
// single MAC or IP.
// Only works on root containers at present, though in the future we could
// extend this to stop + restart slirp4netns
func (r *Runtime) reloadContainerNetwork(ctr *Container) (map[string]types.StatusBlock, error) {
	if ctr.state.NetNS == nil {
		return nil, errors.Wrapf(define.ErrCtrStateInvalid, "container %s network is not configured, refusing to reload", ctr.ID())
	}
	if err := isBridgeNetMode(ctr.config.NetMode); err != nil {
		return nil, err
	}
	logrus.Infof("Going to reload container %s network", ctr.ID())

	err := r.teardownCNI(ctr)
	if err != nil {
		// teardownCNI will error if the iptables rules do not exists and this is the case after
		// a firewall reload. The purpose of network reload is to recreate the rules if they do
		// not exists so we should not log this specific error as error. This would confuse users otherwise.
		// iptables-legacy and iptables-nft will create different errors make sure to match both.
		b, rerr := regexp.MatchString("Couldn't load target `CNI-[a-f0-9]{24}':No such file or directory|Chain 'CNI-[a-f0-9]{24}' does not exist", err.Error())
		if rerr == nil && !b {
			logrus.Error(err)
		} else {
			logrus.Info(err)
		}
	}

	networkOpts, err := ctr.networks()
	if err != nil {
		return nil, err
	}

	// Set the same network settings as before..
	netStatus := ctr.getNetworkStatus()
	for network, perNetOpts := range networkOpts {
		for name, netInt := range netStatus[network].Interfaces {
			perNetOpts.InterfaceName = name
			perNetOpts.StaticMAC = netInt.MacAddress
			for _, netAddress := range netInt.Subnets {
				perNetOpts.StaticIPs = append(perNetOpts.StaticIPs, netAddress.IPNet.IP)
			}
			// Normally interfaces have a length of 1, only for some special cni configs we could get more.
			// For now just use the first interface to get the ips this should be good enough for most cases.
			break
		}
		networkOpts[network] = perNetOpts
	}
	ctr.perNetworkOpts = networkOpts

	return r.configureNetNS(ctr, ctr.state.NetNS)
}

func getContainerNetIO(ctr *Container) (*netlink.LinkStatistics, error) {
	var netStats *netlink.LinkStatistics

	netNSPath, otherCtr, netPathErr := getContainerNetNS(ctr)
	if netPathErr != nil {
		return nil, netPathErr
	}
	if netNSPath == "" {
		// If netNSPath is empty, it was set as none, and no netNS was set up
		// this is a valid state and thus return no error, nor any statistics
		return nil, nil
	}

	// FIXME get the interface from the container netstatus
	dev := "eth0"
	netMode := ctr.config.NetMode
	if otherCtr != nil {
		netMode = otherCtr.config.NetMode
	}
	if netMode.IsSlirp4netns() {
		dev = "tap0"
	}
	err := ns.WithNetNSPath(netNSPath, func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(dev)
		if err != nil {
			return err
		}
		netStats = link.Attrs().Statistics
		return nil
	})
	return netStats, err
}

// Produce an InspectNetworkSettings containing information on the container
// network.
func (c *Container) getContainerNetworkInfo() (*define.InspectNetworkSettings, error) {
	if c.config.NetNsCtr != "" {
		netNsCtr, err := c.runtime.GetContainer(c.config.NetNsCtr)
		if err != nil {
			return nil, err
		}
		// see https://github.com/containers/podman/issues/10090
		// the container has to be locked for syncContainer()
		netNsCtr.lock.Lock()
		defer netNsCtr.lock.Unlock()
		// Have to sync to ensure that state is populated
		if err := netNsCtr.syncContainer(); err != nil {
			return nil, err
		}
		logrus.Debugf("Container %s shares network namespace, retrieving network info of container %s", c.ID(), c.config.NetNsCtr)

		return netNsCtr.getContainerNetworkInfo()
	}

	settings := new(define.InspectNetworkSettings)
	settings.Ports = makeInspectPortBindings(c.config.PortMappings, c.config.ExposedPorts)

	networks, err := c.networks()
	if err != nil {
		return nil, err
	}

	// We can't do more if the network is down.
	if c.state.NetNS == nil {
		// We still want to make dummy configurations for each CNI net
		// the container joined.
		if len(networks) > 0 {
			settings.Networks = make(map[string]*define.InspectAdditionalNetwork, len(networks))
			for net, opts := range networks {
				cniNet := new(define.InspectAdditionalNetwork)
				cniNet.NetworkID = net
				cniNet.Aliases = opts.Aliases
				settings.Networks[net] = cniNet
			}
		}

		return settings, nil
	}

	// Set network namespace path
	settings.SandboxKey = c.state.NetNS.Path()

	netStatus := c.getNetworkStatus()
	// If this is empty, we're probably slirp4netns
	if len(netStatus) == 0 {
		return settings, nil
	}

	// If we have networks - handle that here
	if len(networks) > 0 {
		if len(networks) != len(netStatus) {
			return nil, errors.Wrapf(define.ErrInternal, "network inspection mismatch: asked to join %d network(s) %v, but have information on %d network(s)", len(networks), networks, len(netStatus))
		}

		settings.Networks = make(map[string]*define.InspectAdditionalNetwork)

		for name, opts := range networks {
			result := netStatus[name]
			addedNet := new(define.InspectAdditionalNetwork)
			addedNet.NetworkID = name

			basicConfig, err := resultToBasicNetworkConfig(result)
			if err != nil {
				return nil, err
			}
			addedNet.Aliases = opts.Aliases

			addedNet.InspectBasicNetworkConfig = basicConfig

			settings.Networks[name] = addedNet
		}

		// if not only the default network is connected we can return here
		// otherwise we have to populate the InspectBasicNetworkConfig settings
		_, isDefaultNet := networks[c.runtime.config.Network.DefaultNetwork]
		if !(len(networks) == 1 && isDefaultNet) {
			return settings, nil
		}
	}

	// If not joining networks, we should have at most 1 result
	if len(netStatus) > 1 {
		return nil, errors.Wrapf(define.ErrInternal, "should have at most 1 network status result if not joining networks, instead got %d", len(netStatus))
	}

	if len(netStatus) == 1 {
		for _, status := range netStatus {
			basicConfig, err := resultToBasicNetworkConfig(status)
			if err != nil {
				return nil, err
			}
			settings.InspectBasicNetworkConfig = basicConfig
		}
	}
	return settings, nil
}

// resultToBasicNetworkConfig produces an InspectBasicNetworkConfig from a CNI
// result
func resultToBasicNetworkConfig(result types.StatusBlock) (define.InspectBasicNetworkConfig, error) {
	config := define.InspectBasicNetworkConfig{}
	for _, netInt := range result.Interfaces {
		for _, netAddress := range netInt.Subnets {
			size, _ := netAddress.IPNet.Mask.Size()
			if netAddress.IPNet.IP.To4() != nil {
				//ipv4
				if config.IPAddress == "" {
					config.IPAddress = netAddress.IPNet.IP.String()
					config.IPPrefixLen = size
					config.Gateway = netAddress.Gateway.String()
				} else {
					config.SecondaryIPAddresses = append(config.SecondaryIPAddresses, define.Address{Addr: netAddress.IPNet.IP.String(), PrefixLength: size})
				}
			} else {
				//ipv6
				if config.GlobalIPv6Address == "" {
					config.GlobalIPv6Address = netAddress.IPNet.IP.String()
					config.GlobalIPv6PrefixLen = size
					config.IPv6Gateway = netAddress.Gateway.String()
				} else {
					config.SecondaryIPv6Addresses = append(config.SecondaryIPv6Addresses, define.Address{Addr: netAddress.IPNet.IP.String(), PrefixLength: size})
				}
			}
		}
		if config.MacAddress == "" {
			config.MacAddress = netInt.MacAddress.String()
		} else {
			config.AdditionalMacAddresses = append(config.AdditionalMacAddresses, netInt.MacAddress.String())
		}
	}
	return config, nil
}

type logrusDebugWriter struct {
	prefix string
}

func (w *logrusDebugWriter) Write(p []byte) (int, error) {
	logrus.Debugf("%s%s", w.prefix, string(p))
	return len(p), nil
}

// NetworkDisconnect removes a container from the network
func (c *Container) NetworkDisconnect(nameOrID, netName string, force bool) error {
	// only the bridge mode supports cni networks
	if err := isBridgeNetMode(c.config.NetMode); err != nil {
		return err
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	networks, err := c.networks()
	if err != nil {
		return err
	}

	// check if network exists and if the input is a ID we get the name
	// CNI only uses names so it is important that we only use the name
	netName, err = c.runtime.normalizeNetworkName(netName)
	if err != nil {
		return err
	}

	_, nameExists := networks[netName]
	if !nameExists && len(networks) > 0 {
		return errors.Errorf("container %s is not connected to network %s", nameOrID, netName)
	}

	if err := c.syncContainer(); err != nil {
		return err
	}
	// get network status before we disconnect
	networkStatus := c.getNetworkStatus()

	if err := c.runtime.state.NetworkDisconnect(c, netName); err != nil {
		return err
	}

	c.newNetworkEvent(events.NetworkDisconnect, netName)
	if !c.ensureState(define.ContainerStateRunning, define.ContainerStateCreated) {
		return nil
	}

	if c.state.NetNS == nil {
		return errors.Wrapf(define.ErrNoNetwork, "unable to disconnect %s from %s", nameOrID, netName)
	}

	opts := types.NetworkOptions{
		ContainerID:   c.config.ID,
		ContainerName: getCNIPodName(c),
	}
	opts.PortMappings = c.convertPortMappings()
	opts.Networks = map[string]types.PerNetworkOptions{
		netName: networks[netName],
	}

	if err := c.runtime.teardownNetwork(c.state.NetNS.Path(), opts); err != nil {
		return err
	}

	// update network status if container is running
	oldStatus, statusExist := networkStatus[netName]
	delete(networkStatus, netName)
	c.state.NetworkStatus = networkStatus
	err = c.save()
	if err != nil {
		return err
	}

	// Reload ports when there are still connected networks, maybe we removed the network interface with the child ip.
	// Reloading without connected networks does not make sense, so we can skip this step.
	if rootless.IsRootless() && len(networkStatus) > 0 {
		if err := c.reloadRootlessRLKPortMapping(); err != nil {
			return err
		}
	}

	// Update resolv.conf if required
	if statusExist {
		stringIPs := make([]string, 0, len(oldStatus.DNSServerIPs))
		for _, ip := range oldStatus.DNSServerIPs {
			stringIPs = append(stringIPs, ip.String())
		}
		if len(stringIPs) == 0 {
			return nil
		}
		logrus.Debugf("Removing DNS Servers %v from resolv.conf", stringIPs)
		if err := c.removeNameserver(stringIPs); err != nil {
			return err
		}
	}

	return nil
}

// ConnectNetwork connects a container to a given network
func (c *Container) NetworkConnect(nameOrID, netName string, netOpts types.PerNetworkOptions) error {
	// only the bridge mode supports cni networks
	if err := isBridgeNetMode(c.config.NetMode); err != nil {
		return err
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	networks, err := c.networks()
	if err != nil {
		return err
	}

	// check if network exists and if the input is a ID we get the name
	// CNI only uses names so it is important that we only use the name
	netName, err = c.runtime.normalizeNetworkName(netName)
	if err != nil {
		return err
	}

	if err := c.syncContainer(); err != nil {
		return err
	}

	// get network status before we connect
	networkStatus := c.getNetworkStatus()

	// always add the short id as alias for docker compat
	netOpts.Aliases = append(netOpts.Aliases, c.config.ID[:12])

	if netOpts.InterfaceName == "" {
		netOpts.InterfaceName = getFreeInterfaceName(networks)
		if netOpts.InterfaceName == "" {
			return errors.New("could not find free network interface name")
		}
	}

	if err := c.runtime.state.NetworkConnect(c, netName, netOpts); err != nil {
		return err
	}
	c.newNetworkEvent(events.NetworkConnect, netName)
	if !c.ensureState(define.ContainerStateRunning, define.ContainerStateCreated) {
		return nil
	}
	if c.state.NetNS == nil {
		return errors.Wrapf(define.ErrNoNetwork, "unable to connect %s to %s", nameOrID, netName)
	}

	opts := types.NetworkOptions{
		ContainerID:   c.config.ID,
		ContainerName: getCNIPodName(c),
	}
	opts.PortMappings = c.convertPortMappings()
	opts.Networks = map[string]types.PerNetworkOptions{
		netName: netOpts,
	}

	results, err := c.runtime.setUpNetwork(c.state.NetNS.Path(), opts)
	if err != nil {
		return err
	}
	if len(results) != 1 {
		return errors.New("when adding aliases, results must be of length 1")
	}

	// update network status
	if networkStatus == nil {
		networkStatus = make(map[string]types.StatusBlock, 1)
	}
	networkStatus[netName] = results[netName]
	c.state.NetworkStatus = networkStatus

	err = c.save()
	if err != nil {
		return err
	}

	// The first network needs a port reload to set the correct child ip for the rootlessport process.
	// Adding a second network does not require a port reload because the child ip is still valid.
	if rootless.IsRootless() && len(networks) == 0 {
		if err := c.reloadRootlessRLKPortMapping(); err != nil {
			return err
		}
	}

	ipv6, err := c.checkForIPv6(networkStatus)
	if err != nil {
		return err
	}

	// Update resolv.conf if required
	stringIPs := make([]string, 0, len(results[netName].DNSServerIPs))
	for _, ip := range results[netName].DNSServerIPs {
		if (ip.To4() == nil) && !ipv6 {
			continue
		}
		stringIPs = append(stringIPs, ip.String())
	}
	if len(stringIPs) == 0 {
		return nil
	}
	logrus.Debugf("Adding DNS Servers %v to resolv.conf", stringIPs)
	if err := c.addNameserver(stringIPs); err != nil {
		return err
	}

	return nil
}

// get a free interface name for a new network
// return an empty string if no free name was found
func getFreeInterfaceName(networks map[string]types.PerNetworkOptions) string {
	ifNames := make([]string, 0, len(networks))
	for _, opts := range networks {
		ifNames = append(ifNames, opts.InterfaceName)
	}
	for i := 0; i < 100000; i++ {
		ifName := fmt.Sprintf("eth%d", i)
		if !util.StringInSlice(ifName, ifNames) {
			return ifName
		}
	}
	return ""
}

// DisconnectContainerFromNetwork removes a container from its CNI network
func (r *Runtime) DisconnectContainerFromNetwork(nameOrID, netName string, force bool) error {
	ctr, err := r.LookupContainer(nameOrID)
	if err != nil {
		return err
	}
	return ctr.NetworkDisconnect(nameOrID, netName, force)
}

// ConnectContainerToNetwork connects a container to a CNI network
func (r *Runtime) ConnectContainerToNetwork(nameOrID, netName string, netOpts types.PerNetworkOptions) error {
	ctr, err := r.LookupContainer(nameOrID)
	if err != nil {
		return err
	}
	return ctr.NetworkConnect(nameOrID, netName, netOpts)
}

// normalizeNetworkName takes a network name, a partial or a full network ID and returns the network name.
// If the network is not found a errors is returned.
func (r *Runtime) normalizeNetworkName(nameOrID string) (string, error) {
	net, err := r.network.NetworkInspect(nameOrID)
	if err != nil {
		return "", err
	}
	return net.Name, nil
}

// ocicniPortsToNetTypesPorts convert the old port format to the new one
// while deduplicating ports into ranges
func ocicniPortsToNetTypesPorts(ports []types.OCICNIPortMapping) []types.PortMapping {
	if len(ports) == 0 {
		return nil
	}

	newPorts := make([]types.PortMapping, 0, len(ports))

	// first sort the ports
	sort.Slice(ports, func(i, j int) bool {
		return compareOCICNIPorts(ports[i], ports[j])
	})

	// we already check if the slice is empty so we can use the first element
	currentPort := types.PortMapping{
		HostIP:        ports[0].HostIP,
		HostPort:      uint16(ports[0].HostPort),
		ContainerPort: uint16(ports[0].ContainerPort),
		Protocol:      ports[0].Protocol,
		Range:         1,
	}

	for i := 1; i < len(ports); i++ {
		if ports[i].HostIP == currentPort.HostIP &&
			ports[i].Protocol == currentPort.Protocol &&
			ports[i].HostPort-int32(currentPort.Range) == int32(currentPort.HostPort) &&
			ports[i].ContainerPort-int32(currentPort.Range) == int32(currentPort.ContainerPort) {
			currentPort.Range = currentPort.Range + 1
		} else {
			newPorts = append(newPorts, currentPort)
			currentPort = types.PortMapping{
				HostIP:        ports[i].HostIP,
				HostPort:      uint16(ports[i].HostPort),
				ContainerPort: uint16(ports[i].ContainerPort),
				Protocol:      ports[i].Protocol,
				Range:         1,
			}
		}
	}
	newPorts = append(newPorts, currentPort)
	return newPorts
}

// compareOCICNIPorts will sort the ocicni ports by
// 1) host ip
// 2) protocol
// 3) hostPort
// 4) container port
func compareOCICNIPorts(i, j types.OCICNIPortMapping) bool {
	if i.HostIP != j.HostIP {
		return i.HostIP < j.HostIP
	}

	if i.Protocol != j.Protocol {
		return i.Protocol < j.Protocol
	}

	if i.HostPort != j.HostPort {
		return i.HostPort < j.HostPort
	}

	return i.ContainerPort < j.ContainerPort
}
