//go:build linux
// +build linux

package libpod

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containers/common/libnetwork/resolvconf"
	"github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/netns"
	"github.com/containers/common/pkg/util"
	"github.com/containers/podman/v4/pkg/errorhandling"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/utils"
	"github.com/containers/storage/pkg/lockfile"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/selinux/go-selinux/label"
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

type RootlessNetNS struct {
	ns   ns.NetNS
	dir  string
	Lock *lockfile.LockFile
}

// getPath will join the given path to the rootless netns dir
func (r *RootlessNetNS) getPath(path string) string {
	return filepath.Join(r.dir, path)
}

// Do - run the given function in the rootless netns.
// It does not lock the rootlessCNI lock, the caller
// should only lock when needed, e.g. for network operations.
func (r *RootlessNetNS) Do(toRun func() error) error {
	err := r.ns.Do(func(_ ns.NetNS) error {
		// Before we can run the given function,
		// we have to set up all mounts correctly.

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
			return fmt.Errorf("cannot create a new mount namespace: %w", err)
		}

		xdgRuntimeDir, err := util.GetRuntimeDir()
		if err != nil {
			return fmt.Errorf("could not get runtime directory: %w", err)
		}
		newXDGRuntimeDir := r.getPath(xdgRuntimeDir)
		// 1. Mount the netns into the new run to keep them accessible.
		// Otherwise cni setup will fail because it cannot access the netns files.
		err = unix.Mount(xdgRuntimeDir, newXDGRuntimeDir, "none", unix.MS_BIND|unix.MS_SHARED|unix.MS_REC, "")
		if err != nil {
			return fmt.Errorf("failed to mount runtime directory for rootless netns: %w", err)
		}

		// 2. Also keep /run/systemd if it exists.
		// Many files are symlinked into this dir, for example /dev/log.
		runSystemd := "/run/systemd"
		_, err = os.Stat(runSystemd)
		if err == nil {
			newRunSystemd := r.getPath(runSystemd)
			err = unix.Mount(runSystemd, newRunSystemd, "none", unix.MS_BIND|unix.MS_REC, "")
			if err != nil {
				return fmt.Errorf("failed to mount /run/systemd directory for rootless netns: %w", err)
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
				return fmt.Errorf("failed to stat resolv.conf path: %w", err)
			}

			// no link, just continue
			if fi.Mode()&os.ModeSymlink == 0 {
				continue
			}

			link, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("failed to read resolv.conf symlink: %w", err)
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
				return fmt.Errorf("failed to mount tmpfs on %q for rootless netns: %w", rsr, err)
			}
		}
		if strings.HasPrefix(resolvePath, "/run/") {
			resolvePath = r.getPath(resolvePath)
			err = os.MkdirAll(filepath.Dir(resolvePath), 0700)
			if err != nil {
				return fmt.Errorf("failed to create rootless-netns resolv.conf directory: %w", err)
			}
			// we want to bind mount on this file so we have to create the file first
			_, err = os.OpenFile(resolvePath, os.O_CREATE|os.O_RDONLY, 0700)
			if err != nil {
				return fmt.Errorf("failed to create rootless-netns resolv.conf file: %w", err)
			}
		}
		// mount resolv.conf to make use of the host dns
		err = unix.Mount(r.getPath("resolv.conf"), resolvePath, "none", unix.MS_BIND, "")
		if err != nil {
			return fmt.Errorf("failed to mount resolv.conf for rootless netns: %w", err)
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
			return fmt.Errorf("failed to mount %s for rootless netns: %w", varTarget, err)
		}

		// 5. Mount the new prepared run dir to /run, it has to be recursive to keep the other bind mounts.
		runDir := r.getPath("run")
		err = unix.Mount(runDir, "/run", "none", unix.MS_BIND|unix.MS_REC, "")
		if err != nil {
			return fmt.Errorf("failed to mount /run for rootless netns: %w", err)
		}

		// run the given function in the correct namespace
		err = toRun()
		return err
	})
	return err
}

// Clean up the rootless network namespace if needed.
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
		// rootless netns lock and the container lock
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
		return c.state.NetNS != ""
	}
	ctrs, err := runtime.GetContainers(false, activeNetns)
	if err != nil {
		return err
	}
	// no cleanup if we found no other containers with a netns
	// we will always find one container (the container cleanup that is currently calling us)
	if len(ctrs) > 1 {
		return nil
	}
	logrus.Debug("Cleaning up rootless network namespace")
	err = netns.UnmountNS(r.ns.Path())
	if err != nil {
		return err
	}
	// make the following errors not fatal
	err = r.ns.Close()
	if err != nil {
		logrus.Error(err)
	}
	b, err := os.ReadFile(r.getPath(rootlessNetNsSilrp4netnsPidFile))
	if err == nil {
		var i int
		i, err = strconv.Atoi(string(b))
		if err == nil {
			// kill the slirp process so we do not leak it
			err = syscall.Kill(i, syscall.SIGTERM)
		}
	}
	if err != nil {
		logrus.Errorf("Failed to kill slirp4netns process: %v", err)
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
	lock, err := lockfile.GetLockFile(lfile)
	if err != nil {
		return nil, fmt.Errorf("failed to get rootless-netns lockfile: %w", err)
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
		return nil, fmt.Errorf("could not create rootless-netns directory: %w", err)
	}

	nsDir, err := netns.GetNSRunDir()
	if err != nil {
		return nil, err
	}

	// create a hash from the static dir
	// the cleanup will check if there are running containers
	// if you run a several libpod instances with different root/runroot directories this check will fail
	// we want one netns for each libpod static dir so we use the hash to prevent name collisions
	hash := sha256.Sum256([]byte(r.config.Engine.StaticDir))
	netnsName := fmt.Sprintf("%s-%x", rootlessNetNsName, hash[:10])

	path := filepath.Join(nsDir, netnsName)
	ns, err := ns.GetNS(path)
	if err != nil {
		if !new {
			// return an error if we could not get the namespace and should no create one
			return nil, fmt.Errorf("getting rootless network namespace: %w", err)
		}
		// create a new namespace
		logrus.Debugf("creating rootless network namespace with name %q", netnsName)
		ns, err = netns.NewNSWithName(netnsName)
		if err != nil {
			return nil, fmt.Errorf("creating rootless network namespace: %w", err)
		}
		// set up slirp4netns here
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
			return nil, fmt.Errorf("failed to open pipe: %w", err)
		}
		defer errorhandling.CloseQuiet(syncR)
		defer errorhandling.CloseQuiet(syncW)

		netOptions, err := parseSlirp4netnsNetworkOptions(r, nil)
		if err != nil {
			return nil, err
		}
		slirpFeatures, err := checkSlirpFlags(path)
		if err != nil {
			return nil, fmt.Errorf("checking slirp4netns binary %s: %q: %w", path, err, err)
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
			return nil, fmt.Errorf("failed to open slirp4netns log file %s: %w", logPath, err)
		}
		defer logFile.Close()
		// Unlink immediately the file so we won't need to worry about cleaning it up later.
		// It is still accessible through the open fd logFile.
		if err := os.Remove(logPath); err != nil {
			return nil, fmt.Errorf("delete file %s: %w", logPath, err)
		}
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("failed to start slirp4netns process: %w", err)
		}
		// create pid file for the slirp4netns process
		// this is need to kill the process in the cleanup
		pid := strconv.Itoa(cmd.Process.Pid)
		err = os.WriteFile(filepath.Join(rootlessNetNsDir, rootlessNetNsSilrp4netnsPidFile), []byte(pid), 0700)
		if err != nil {
			return nil, fmt.Errorf("unable to write rootless-netns slirp4netns pid file: %w", err)
		}

		defer func() {
			if err := cmd.Process.Release(); err != nil {
				logrus.Errorf("Unable to release command process: %q", err)
			}
		}()

		if err := waitForSync(syncR, cmd, logFile, 1*time.Second); err != nil {
			return nil, err
		}

		if utils.RunsOnSystemd() {
			// move to systemd scope to prevent systemd from killing it
			err = utils.MoveRootlessNetnsSlirpProcessToUserSlice(cmd.Process.Pid)
			if err != nil {
				// only log this, it is not fatal but can lead to issues when running podman inside systemd units
				logrus.Errorf("failed to move the rootless netns slirp4netns process to the systemd user.slice: %v", err)
			}
		}

		// build a new resolv.conf file which uses the slirp4netns dns server address
		resolveIP, err := GetSlirp4netnsDNS(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to determine default slirp4netns DNS address: %w", err)
		}

		if netOptions.cidr != "" {
			_, cidr, err := net.ParseCIDR(netOptions.cidr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse slirp4netns cidr: %w", err)
			}
			resolveIP, err = GetSlirp4netnsDNS(cidr)
			if err != nil {
				return nil, fmt.Errorf("failed to determine slirp4netns DNS address from cidr: %s: %w", cidr.String(), err)
			}
		}

		if err := resolvconf.New(&resolvconf.Params{
			Path: filepath.Join(rootlessNetNsDir, "resolv.conf"),
			// fake the netns since we want to filter localhost
			Namespaces: []specs.LinuxNamespace{
				{Type: specs.NetworkNamespace},
			},
			IPv6Enabled:     netOptions.enableIPv6,
			KeepHostServers: true,
			Nameservers:     []string{resolveIP.String()},
		}); err != nil {
			return nil, fmt.Errorf("failed to create rootless netns resolv.conf: %w", err)
		}
		// create cni directories to store files
		// they will be bind mounted to the correct location in an extra mount ns
		err = os.MkdirAll(filepath.Join(rootlessNetNsDir, persistentCNIDir), 0700)
		if err != nil {
			return nil, fmt.Errorf("could not create rootless-netns var directory: %w", err)
		}
		runDir := filepath.Join(rootlessNetNsDir, "run")
		err = os.MkdirAll(runDir, 0700)
		if err != nil {
			return nil, fmt.Errorf("could not create rootless-netns run directory: %w", err)
		}
		// relabel the new run directory to the iptables /run label
		// this is important, otherwise the iptables command will fail
		err = label.Relabel(runDir, "system_u:object_r:iptables_var_run_t:s0", false)
		if err != nil {
			return nil, fmt.Errorf("could not create relabel rootless-netns run directory: %w", err)
		}
		// create systemd run directory
		err = os.MkdirAll(filepath.Join(runDir, "systemd"), 0700)
		if err != nil {
			return nil, fmt.Errorf("could not create rootless-netns systemd directory: %w", err)
		}
		// create the directory for the netns files at the same location
		// relative to the rootless-netns location
		err = os.MkdirAll(filepath.Join(rootlessNetNsDir, nsDir), 0700)
		if err != nil {
			return nil, fmt.Errorf("could not create rootless-netns netns directory: %w", err)
		}
	}

	// The CNI plugins and netavark need access to iptables in $PATH. As it turns out debian doesn't put
	// /usr/sbin in $PATH for rootless users. This will break rootless networking completely.
	// We might break existing users and we cannot expect everyone to change their $PATH so
	// lets add /usr/sbin to $PATH ourselves.
	path = os.Getenv("PATH")
	if !strings.Contains(path, "/usr/sbin") {
		path += ":/usr/sbin"
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

// Create and configure a new network namespace for a container
func (r *Runtime) configureNetNS(ctr *Container, ctrNS string) (status map[string]types.StatusBlock, rerr error) {
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
	if ctr.config.NetMode.IsPasta() {
		return nil, r.setupPasta(ctr, ctrNS)
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

	netOpts := ctr.getNetworkOptions(networks)
	netStatus, err := r.setUpNetwork(ctrNS, netOpts)
	if err != nil {
		return nil, err
	}

	// set up rootless port forwarder when rootless with ports and the network status is empty,
	// if this is called from network reload the network status will not be empty and we should
	// not set up port because they are still active
	if rootless.IsRootless() && len(ctr.config.PortMappings) > 0 && ctr.getNetworkStatus() == nil {
		// set up port forwarder for rootless netns
		// TODO: support slirp4netns port forwarder as well
		// make sure to fix this in container.handleRestartPolicy() as well
		// Important we have to call this after r.setUpNetwork() so that
		// we can use the proper netStatus
		err = r.setupRootlessPortMappingViaRLK(ctr, ctrNS, netStatus)
	}
	return netStatus, err
}

// Create and configure a new network namespace for a container
func (r *Runtime) createNetNS(ctr *Container) (n string, q map[string]types.StatusBlock, retErr error) {
	ctrNS, err := netns.NewNS()
	if err != nil {
		return "", nil, fmt.Errorf("creating network namespace for container %s: %w", ctr.ID(), err)
	}
	defer func() {
		if retErr != nil {
			if err := netns.UnmountNS(ctrNS.Path()); err != nil {
				logrus.Errorf("Unmounting partially created network namespace for container %s: %v", ctr.ID(), err)
			}
			if err := ctrNS.Close(); err != nil {
				logrus.Errorf("Closing partially created network namespace for container %s: %v", ctr.ID(), err)
			}
		}
	}()

	logrus.Debugf("Made network namespace at %s for container %s", ctrNS.Path(), ctr.ID())

	var networkStatus map[string]types.StatusBlock
	networkStatus, err = r.configureNetNS(ctr, ctrNS.Path())
	return ctrNS.Path(), networkStatus, err
}

// Configure the network namespace using the container process
func (r *Runtime) setupNetNS(ctr *Container) error {
	nsProcess := fmt.Sprintf("/proc/%d/ns/net", ctr.state.PID)

	b := make([]byte, 16)

	if _, err := rand.Reader.Read(b); err != nil {
		return fmt.Errorf("failed to generate random netns name: %w", err)
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
		return fmt.Errorf("cannot mount %s: %w", nsPath, err)
	}

	networkStatus, err := r.configureNetNS(ctr, nsPath)

	// Assign NetNS attributes to container
	ctr.state.NetNS = nsPath
	ctr.state.NetworkStatus = networkStatus
	return err
}

// Tear down a network namespace, undoing all state associated with it.
func (r *Runtime) teardownNetNS(ctr *Container) error {
	if err := r.unexposeMachinePorts(ctr.config.PortMappings); err != nil {
		// do not return an error otherwise we would prevent network cleanup
		logrus.Errorf("failed to free gvproxy machine ports: %v", err)
	}

	// Do not check the error here, we want to always umount the netns
	// This will ensure that the container interface will be deleted
	// even when there is a CNI or netavark bug.
	prevErr := r.teardownNetwork(ctr)

	// First unmount the namespace
	if err := netns.UnmountNS(ctr.state.NetNS); err != nil {
		if prevErr != nil {
			logrus.Error(prevErr)
		}
		return fmt.Errorf("unmounting network namespace for container %s: %w", ctr.ID(), err)
	}

	ctr.state.NetNS = ""

	return prevErr
}

func getContainerNetNS(ctr *Container) (string, *Container, error) {
	if ctr.state.NetNS != "" {
		return ctr.state.NetNS, nil, nil
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

// TODO (5.0): return the statistics per network interface
// This would allow better compat with docker.
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

	netMode := ctr.config.NetMode
	netStatus := ctr.getNetworkStatus()
	if otherCtr != nil {
		netMode = otherCtr.config.NetMode
		netStatus = otherCtr.getNetworkStatus()
	}
	if netMode.IsSlirp4netns() {
		// create a fake status with correct interface name for the logic below
		netStatus = map[string]types.StatusBlock{
			"slirp4netns": {
				Interfaces: map[string]types.NetInterface{"tap0": {}},
			},
		}
	}
	err := ns.WithNetNSPath(netNSPath, func(_ ns.NetNS) error {
		for _, status := range netStatus {
			for dev := range status.Interfaces {
				link, err := netlink.LinkByName(dev)
				if err != nil {
					return err
				}
				if netStats == nil {
					netStats = link.Attrs().Statistics
					continue
				}
				// Currently only Tx/RxBytes are used.
				// In the future we should return all stats per interface so that
				// api users have a better options.
				stats := link.Attrs().Statistics
				netStats.TxBytes += stats.TxBytes
				netStats.RxBytes += stats.RxBytes
			}
		}
		return nil
	})
	return netStats, err
}

func (c *Container) joinedNetworkNSPath() string {
	for _, namespace := range c.config.Spec.Linux.Namespaces {
		if namespace.Type == specs.NetworkNamespace {
			return namespace.Path
		}
	}
	return ""
}

func (c *Container) inspectJoinedNetworkNS(networkns string) (q types.StatusBlock, retErr error) {
	var result types.StatusBlock
	err := ns.WithNetNSPath(networkns, func(_ ns.NetNS) error {
		ifaces, err := net.Interfaces()
		if err != nil {
			return err
		}
		routes, err := netlink.RouteList(nil, netlink.FAMILY_ALL)
		if err != nil {
			return err
		}
		var gateway net.IP
		for _, route := range routes {
			// default gateway
			if route.Dst == nil {
				gateway = route.Gw
			}
		}
		result.Interfaces = make(map[string]types.NetInterface)
		for _, iface := range ifaces {
			if iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			if len(addrs) == 0 {
				continue
			}
			subnets := make([]types.NetAddress, 0, len(addrs))
			for _, address := range addrs {
				if ipnet, ok := address.(*net.IPNet); ok {
					if ipnet.IP.IsLinkLocalMulticast() || ipnet.IP.IsLinkLocalUnicast() {
						continue
					}
					subnet := types.NetAddress{
						IPNet: types.IPNet{
							IPNet: *ipnet,
						},
					}
					if ipnet.Contains(gateway) {
						subnet.Gateway = gateway
					}
					subnets = append(subnets, subnet)
				}
			}
			result.Interfaces[iface.Name] = types.NetInterface{
				Subnets:    subnets,
				MacAddress: types.HardwareAddr(iface.HardwareAddr),
			}
		}
		return nil
	})
	return result, err
}

type logrusDebugWriter struct {
	prefix string
}

func (w *logrusDebugWriter) Write(p []byte) (int, error) {
	logrus.Debugf("%s%s", w.prefix, string(p))
	return len(p), nil
}
