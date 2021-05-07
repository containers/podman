// +build linux

package libpod

import (
	"crypto/rand"
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

	cnitypes "github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/libpod/events"
	"github.com/containers/podman/v3/libpod/network"
	"github.com/containers/podman/v3/pkg/errorhandling"
	"github.com/containers/podman/v3/pkg/namespaces"
	"github.com/containers/podman/v3/pkg/netns"
	"github.com/containers/podman/v3/pkg/resolvconf"
	"github.com/containers/podman/v3/pkg/rootless"
	"github.com/containers/podman/v3/pkg/util"
	"github.com/containers/storage/pkg/lockfile"
	"github.com/cri-o/ocicni/pkg/ocicni"
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

	// rootlessCNINSName is the file name for the rootless network namespace bind mount
	rootlessCNINSName = "rootless-cni-ns"
)

// Get an OCICNI network config
func (r *Runtime) getPodNetwork(id, name, nsPath string, networks []string, ports []ocicni.PortMapping, staticIP net.IP, staticMAC net.HardwareAddr, netDescriptions ContainerNetworkDescriptions) ocicni.PodNetwork {
	var networkKey string
	if len(networks) > 0 {
		// This is inconsistent for >1 ctrNetwork, but it's probably the
		// best we can do.
		networkKey = networks[0]
	} else {
		networkKey = r.netPlugin.GetDefaultNetworkName()
	}
	ctrNetwork := ocicni.PodNetwork{
		Name:      name,
		Namespace: name, // TODO is there something else we should put here? We don't know about Kube namespaces
		ID:        id,
		NetNS:     nsPath,
		RuntimeConfig: map[string]ocicni.RuntimeConfig{
			networkKey: {PortMappings: ports},
		},
	}

	// If we have extra networks, add them
	if len(networks) > 0 {
		ctrNetwork.Networks = make([]ocicni.NetAttachment, len(networks))
		for i, netName := range networks {
			ctrNetwork.Networks[i].Name = netName
			if eth, exists := netDescriptions.getInterfaceByName(netName); exists {
				ctrNetwork.Networks[i].Ifname = eth
			}
		}
	}

	if staticIP != nil || staticMAC != nil {
		// For static IP or MAC, we need to populate networks even if
		// it's just the default.
		if len(networks) == 0 {
			// If len(networks) == 0 this is guaranteed to be the
			// default ctrNetwork.
			ctrNetwork.Networks = []ocicni.NetAttachment{{Name: networkKey}}
		}
		var rt ocicni.RuntimeConfig = ocicni.RuntimeConfig{PortMappings: ports}
		if staticIP != nil {
			rt.IP = staticIP.String()
		}
		if staticMAC != nil {
			rt.MAC = staticMAC.String()
		}
		ctrNetwork.RuntimeConfig = map[string]ocicni.RuntimeConfig{
			networkKey: rt,
		}
	}

	return ctrNetwork
}

type RootlessCNI struct {
	ns   ns.NetNS
	dir  string
	lock lockfile.Locker
}

func (r *RootlessCNI) Do(toRun func() error) error {
	err := r.ns.Do(func(_ ns.NetNS) error {
		// before we can run the given function
		// we have to setup all mounts correctly

		// create a new mount namespace
		// this should happen inside the netns thread
		err := unix.Unshare(unix.CLONE_NEWNS)
		if err != nil {
			return errors.Wrapf(err, "cannot create a new mount namespace")
		}

		netNsDir, err := netns.GetNSRunDir()
		if err != nil {
			return errors.Wrap(err, "could not get network namespace directory")
		}
		newNetNsDir := filepath.Join(r.dir, netNsDir)
		// mount the netns into the new run to keep them accessible
		// otherwise cni setup will fail because it cannot access the netns files
		err = unix.Mount(netNsDir, newNetNsDir, "none", unix.MS_BIND|unix.MS_SHARED|unix.MS_REC, "")
		if err != nil {
			return errors.Wrap(err, "failed to mount netns directory for rootless cni")
		}

		// mount resolv.conf to make use of the host dns
		err = unix.Mount(filepath.Join(r.dir, "resolv.conf"), "/etc/resolv.conf", "none", unix.MS_BIND, "")
		if err != nil {
			return errors.Wrap(err, "failed to mount resolv.conf for rootless cni")
		}

		// also keep /run/systemd if it exists
		// many files are symlinked into this dir, for example /dev/log
		runSystemd := "/run/systemd"
		_, err = os.Stat(runSystemd)
		if err == nil {
			newRunSystemd := filepath.Join(r.dir, runSystemd[1:])
			err = unix.Mount(runSystemd, newRunSystemd, "none", unix.MS_BIND|unix.MS_REC, "")
			if err != nil {
				return errors.Wrap(err, "failed to mount /run/systemd directory for rootless cni")
			}
		}

		// cni plugins need access to /var and /run
		runDir := filepath.Join(r.dir, "run")
		varDir := filepath.Join(r.dir, "var")
		// make sure to mount var first
		err = unix.Mount(varDir, "/var", "none", unix.MS_BIND, "")
		if err != nil {
			return errors.Wrap(err, "failed to mount /var for rootless cni")
		}
		// recursive mount to keep the netns mount
		err = unix.Mount(runDir, "/run", "none", unix.MS_BIND|unix.MS_REC, "")
		if err != nil {
			return errors.Wrap(err, "failed to mount /run for rootless cni")
		}

		// run the given function in the correct namespace
		err = toRun()
		return err
	})
	return err
}

// Cleanup the rootless cni namespace if needed
// check if we have running containers with the bridge network mode
func (r *RootlessCNI) Cleanup(runtime *Runtime) error {
	_, err := os.Stat(r.dir)
	if os.IsNotExist(err) {
		// the directory does not exists no need for cleanup
		return nil
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	running := func(c *Container) bool {
		// we cannot use c.state() because it will try to lock the container
		// using c.state.State directly should be good enough for this use case
		state := c.state.State
		return state == define.ContainerStateRunning
	}
	ctrs, err := runtime.GetContainersWithoutLock(running)
	if err != nil {
		return err
	}
	cleanup := true
	for _, ctr := range ctrs {
		if ctr.config.NetMode.IsBridge() {
			cleanup = false
		}
	}
	if cleanup {
		// make sure the the cni results (cache) dir is empty
		// libpod instances with another root dir are not covered by the check above
		// this allows several libpod instances to use the same rootless cni ns
		contents, err := ioutil.ReadDir(filepath.Join(r.dir, "var/lib/cni/results"))
		if (err == nil && len(contents) == 0) || os.IsNotExist(err) {
			logrus.Debug("Cleaning up rootless cni namespace")
			err = netns.UnmountNS(r.ns)
			if err != nil {
				return err
			}
			// make the following errors not fatal
			err = r.ns.Close()
			if err != nil {
				logrus.Error(err)
			}
			b, err := ioutil.ReadFile(filepath.Join(r.dir, "rootless-cni-slirp4netns.pid"))
			if err == nil {
				var i int
				i, err = strconv.Atoi(string(b))
				if err == nil {
					// kill the slirp process so we do not leak it
					err = syscall.Kill(i, syscall.SIGTERM)
				}
			}
			if err != nil {
				logrus.Errorf("failed to kill slirp4netns process: %s", err)
			}
			err = os.RemoveAll(r.dir)
			if err != nil {
				logrus.Error(err)
			}
		} else if err != nil && !os.IsNotExist(err) {
			logrus.Errorf("could not read rootless cni directory, skipping cleanup: %s", err)
		}
	}
	return nil
}

// GetRootlessCNINetNs returns the rootless cni object. If create is set to true
// the rootless cni namespace will be created if it does not exists already.
func (r *Runtime) GetRootlessCNINetNs(new bool) (*RootlessCNI, error) {
	var rootlessCNINS *RootlessCNI
	if rootless.IsRootless() {
		runDir, err := util.GetRuntimeDir()
		if err != nil {
			return nil, err
		}
		cniDir := filepath.Join(runDir, "rootless-cni")
		err = os.MkdirAll(cniDir, 0700)
		if err != nil {
			return nil, errors.Wrap(err, "could not create rootless-cni directory")
		}

		lfile := filepath.Join(cniDir, "rootless-cni.lck")
		lock, err := lockfile.GetLockfile(lfile)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get rootless-cni lockfile")
		}
		lock.Lock()
		defer lock.Unlock()

		nsDir, err := netns.GetNSRunDir()
		if err != nil {
			return nil, err
		}
		path := filepath.Join(nsDir, rootlessCNINSName)
		ns, err := ns.GetNS(path)
		if err != nil {
			if new {
				// create a new namespace
				logrus.Debug("creating rootless cni network namespace")
				ns, err = netns.NewNSWithName(rootlessCNINSName)
				if err != nil {
					return nil, errors.Wrap(err, "error creating rootless cni network namespace")
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

				logPath := filepath.Join(r.config.Engine.TmpDir, "slirp4netns-rootless-cni.log")
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
				err = ioutil.WriteFile(filepath.Join(cniDir, "rootless-cni-slirp4netns.pid"), []byte(pid), 0700)
				if err != nil {
					errors.Wrap(err, "unable to write rootless-cni slirp4netns pid file")
				}

				defer func() {
					if err := cmd.Process.Release(); err != nil {
						logrus.Errorf("unable to release command process: %q", err)
					}
				}()

				if err := waitForSync(syncR, cmd, logFile, 1*time.Second); err != nil {
					return nil, err
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
				searchDomains := resolvconf.GetSearchDomains(conf.Content)
				dnsOptions := resolvconf.GetOptions(conf.Content)

				_, err = resolvconf.Build(filepath.Join(cniDir, "resolv.conf"), []string{resolveIP.String()}, searchDomains, dnsOptions)
				if err != nil {
					return nil, errors.Wrap(err, "failed to create rootless cni resolv.conf")
				}

				// create cni directories to store files
				// they will be bind mounted to the correct location in a extra mount ns
				err = os.MkdirAll(filepath.Join(cniDir, "var"), 0700)
				if err != nil {
					return nil, errors.Wrap(err, "could not create rootless-cni var directory")
				}
				runDir := filepath.Join(cniDir, "run")
				err = os.MkdirAll(runDir, 0700)
				if err != nil {
					return nil, errors.Wrap(err, "could not create rootless-cni run directory")
				}
				// relabel the new run directory to the iptables /run label
				// this is important, otherwise the iptables command will fail
				err = label.Relabel(runDir, "system_u:object_r:iptables_var_run_t:s0", false)
				if err != nil {
					return nil, errors.Wrap(err, "could not create relabel rootless-cni run directory")
				}
				// create systemd run directory
				err = os.MkdirAll(filepath.Join(runDir, "systemd"), 0700)
				if err != nil {
					return nil, errors.Wrap(err, "could not create rootless-cni systemd directory")
				}
				// create the directory for the netns files at the same location
				// relative to the rootless-cni location
				err = os.MkdirAll(filepath.Join(cniDir, nsDir), 0700)
				if err != nil {
					return nil, errors.Wrap(err, "could not create rootless-cni netns directory")
				}
			} else {
				// return a error if we could not get the namespace and should no create one
				return nil, errors.Wrap(err, "error getting rootless cni network namespace")
			}
		}

		// The CNI plugins need access to iptables in $PATH. As it turns out debian doesn't put
		// /usr/sbin in $PATH for rootless users. This will break rootless cni completely.
		// We might break existing users and we cannot expect everyone to change their $PATH so
		// lets add /usr/sbin to $PATH ourselves.
		path = os.Getenv("PATH")
		if !strings.Contains(path, "/usr/sbin") {
			path = path + ":/usr/sbin"
			os.Setenv("PATH", path)
		}

		rootlessCNINS = &RootlessCNI{
			ns:   ns,
			dir:  cniDir,
			lock: lock,
		}
	}
	return rootlessCNINS, nil
}

// setPrimaryMachineIP is used for podman-machine and it sets
// and environment variable with the IP address of the podman-machine
// host.
func setPrimaryMachineIP() error {
	// no connection is actually made here
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return err
	}
	defer func() {
		if err := conn.Close(); err != nil {
			logrus.Error(err)
		}
	}()
	addr := conn.LocalAddr().(*net.UDPAddr)
	return os.Setenv("PODMAN_MACHINE_HOST", addr.IP.String())
}

// setUpOCICNIPod will set up the cni networks, on error it will also tear down the cni
// networks. If rootless it will join/create the rootless cni namespace.
func (r *Runtime) setUpOCICNIPod(podNetwork ocicni.PodNetwork) ([]ocicni.NetResult, error) {
	if r.config.MachineEnabled() {
		if err := setPrimaryMachineIP(); err != nil {
			return nil, err
		}
	}
	rootlessCNINS, err := r.GetRootlessCNINetNs(true)
	if err != nil {
		return nil, err
	}
	var results []ocicni.NetResult
	setUpPod := func() error {
		results, err = r.netPlugin.SetUpPod(podNetwork)
		if err != nil {
			if err2 := r.netPlugin.TearDownPod(podNetwork); err2 != nil {
				logrus.Errorf("Error tearing down partially created network namespace for container %s: %v", podNetwork.ID, err2)
			}
			return errors.Wrapf(err, "error configuring network namespace for container %s", podNetwork.ID)
		}
		return nil
	}
	// rootlessCNINS is nil if we are root
	if rootlessCNINS != nil {
		// execute the cni setup in the rootless net ns
		err = rootlessCNINS.Do(setUpPod)
	} else {
		err = setUpPod()
	}
	return results, err
}

// getCNIPodName return the pod name (hostname) used by CNI and the dnsname plugin.
// If we are in the pod network namespace use the pod name otherwise the container name
func getCNIPodName(c *Container) string {
	if c.config.NetMode.IsPod() || c.IsInfra() {
		pod, err := c.runtime.GetPod(c.PodID())
		if err == nil {
			return pod.Name()
		}
	}
	return c.Name()
}

// Create and configure a new network namespace for a container
func (r *Runtime) configureNetNS(ctr *Container, ctrNS ns.NetNS) ([]*cnitypes.Result, error) {
	var requestedIP net.IP
	if ctr.requestedIP != nil {
		requestedIP = ctr.requestedIP
		// cancel request for a specific IP in case the container is reused later
		ctr.requestedIP = nil
	} else {
		requestedIP = ctr.config.StaticIP
	}

	var requestedMAC net.HardwareAddr
	if ctr.requestedMAC != nil {
		requestedMAC = ctr.requestedMAC
		// cancel request for a specific MAC in case the container is reused later
		ctr.requestedMAC = nil
	} else {
		requestedMAC = ctr.config.StaticMAC
	}

	podName := getCNIPodName(ctr)

	networks, _, err := ctr.networks()
	if err != nil {
		return nil, err
	}
	// All networks have been removed from the container.
	// This is effectively forcing net=none.
	if len(networks) == 0 {
		return nil, nil
	}

	// Update container map of interface descriptions
	if err := ctr.setupNetworkDescriptions(networks); err != nil {
		return nil, err
	}
	podNetwork := r.getPodNetwork(ctr.ID(), podName, ctrNS.Path(), networks, ctr.config.PortMappings, requestedIP, requestedMAC, ctr.state.NetInterfaceDescriptions)
	aliases, err := ctr.runtime.state.GetAllNetworkAliases(ctr)
	if err != nil {
		return nil, err
	}
	if len(aliases) > 0 {
		podNetwork.Aliases = aliases
	}

	results, err := r.setUpOCICNIPod(podNetwork)
	if err != nil {
		return nil, err
	}

	networkStatus := make([]*cnitypes.Result, 0)
	for idx, r := range results {
		logrus.Debugf("[%d] CNI result: %v", idx, r.Result)
		resultCurrent, err := cnitypes.GetResult(r.Result)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing CNI plugin result %q: %v", r.Result, err)
		}
		networkStatus = append(networkStatus, resultCurrent)
	}

	return networkStatus, nil
}

// Create and configure a new network namespace for a container
func (r *Runtime) createNetNS(ctr *Container) (n ns.NetNS, q []*cnitypes.Result, retErr error) {
	ctrNS, err := netns.NewNS()
	if err != nil {
		return nil, nil, errors.Wrapf(err, "error creating network namespace for container %s", ctr.ID())
	}
	defer func() {
		if retErr != nil {
			if err := netns.UnmountNS(ctrNS); err != nil {
				logrus.Errorf("Error unmounting partially created network namespace for container %s: %v", ctr.ID(), err)
			}
			if err := ctrNS.Close(); err != nil {
				logrus.Errorf("Error closing partially created network namespace for container %s: %v", ctr.ID(), err)
			}
		}
	}()

	logrus.Debugf("Made network namespace at %s for container %s", ctrNS.Path(), ctr.ID())

	networkStatus := []*cnitypes.Result{}
	if !ctr.config.NetMode.IsSlirp4netns() {
		networkStatus, err = r.configureNetNS(ctr, ctrNS)
	}
	return ctrNS, networkStatus, err
}

// Configure the network namespace for a rootless container
func (r *Runtime) setupRootlessNetNS(ctr *Container) error {
	if ctr.config.NetMode.IsSlirp4netns() {
		return r.setupSlirp4netns(ctr)
	}
	networks, _, err := ctr.networks()
	if err != nil {
		return err
	}
	if len(networks) > 0 && len(ctr.config.PortMappings) > 0 {
		// set up port forwarder for CNI-in-slirp4netns
		netnsPath := ctr.state.NetNS.Path()
		// TODO: support slirp4netns port forwarder as well
		return r.setupRootlessPortMappingViaRLK(ctr, netnsPath)
	}
	return nil
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
	nsPath = filepath.Join(nsPath, fmt.Sprintf("cni-%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]))

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

// Tear down a container's CNI network configuration and joins the
// rootless net ns as rootless user
func (r *Runtime) teardownOCICNIPod(podNetwork ocicni.PodNetwork) error {
	rootlessCNINS, err := r.GetRootlessCNINetNs(false)
	if err != nil {
		return err
	}
	tearDownPod := func() error {
		err := r.netPlugin.TearDownPod(podNetwork)
		return errors.Wrapf(err, "error tearing down CNI namespace configuration for container %s", podNetwork.ID)
	}

	// rootlessCNINS is nil if we are root
	if rootlessCNINS != nil {
		// execute the cni setup in the rootless net ns
		err = rootlessCNINS.Do(tearDownPod)
		if err == nil {
			err = rootlessCNINS.Cleanup(r)
		}
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

	networks, _, err := ctr.networks()
	if err != nil {
		return err
	}

	if !ctr.config.NetMode.IsSlirp4netns() && len(networks) > 0 {
		var requestedIP net.IP
		if ctr.requestedIP != nil {
			requestedIP = ctr.requestedIP
			// cancel request for a specific IP in case the container is reused later
			ctr.requestedIP = nil
		} else {
			requestedIP = ctr.config.StaticIP
		}

		var requestedMAC net.HardwareAddr
		if ctr.requestedMAC != nil {
			requestedMAC = ctr.requestedMAC
			// cancel request for a specific MAC in case the container is reused later
			ctr.requestedMAC = nil
		} else {
			requestedMAC = ctr.config.StaticMAC
		}

		podNetwork := r.getPodNetwork(ctr.ID(), ctr.Name(), ctr.state.NetNS.Path(), networks, ctr.config.PortMappings, requestedIP, requestedMAC, ctr.state.NetInterfaceDescriptions)
		err = r.teardownOCICNIPod(podNetwork)
		return err
	}
	return nil
}

// Tear down a network namespace, undoing all state associated with it.
func (r *Runtime) teardownNetNS(ctr *Container) error {
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

func getContainerNetNS(ctr *Container) (string, error) {
	if ctr.state.NetNS != nil {
		return ctr.state.NetNS.Path(), nil
	}
	if ctr.config.NetNsCtr != "" {
		c, err := ctr.runtime.GetContainer(ctr.config.NetNsCtr)
		if err != nil {
			return "", err
		}
		if err = c.syncContainer(); err != nil {
			return "", err
		}
		return getContainerNetNS(c)
	}
	return "", nil
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
func (r *Runtime) reloadContainerNetwork(ctr *Container) ([]*cnitypes.Result, error) {
	if ctr.state.NetNS == nil {
		return nil, errors.Wrapf(define.ErrCtrStateInvalid, "container %s network is not configured, refusing to reload", ctr.ID())
	}
	if err := isBridgeNetMode(ctr.config.NetMode); err != nil {
		return nil, err
	}

	logrus.Infof("Going to reload container %s network", ctr.ID())

	var requestedIP net.IP
	var requestedMAC net.HardwareAddr
	// Set requested IP and MAC address, if possible.
	if len(ctr.state.NetworkStatus) == 1 {
		result := ctr.state.NetworkStatus[0]
		if len(result.IPs) == 1 {
			resIP := result.IPs[0]

			requestedIP = resIP.Address.IP
			ctr.requestedIP = requestedIP
			logrus.Debugf("Going to preserve container %s IP address %s", ctr.ID(), ctr.requestedIP.String())

			if resIP.Interface != nil && *resIP.Interface < len(result.Interfaces) && *resIP.Interface >= 0 {
				var err error
				requestedMAC, err = net.ParseMAC(result.Interfaces[*resIP.Interface].Mac)
				if err != nil {
					return nil, errors.Wrapf(err, "error parsing container %s MAC address %s", ctr.ID(), result.Interfaces[*resIP.Interface].Mac)
				}
				ctr.requestedMAC = requestedMAC
				logrus.Debugf("Going to preserve container %s MAC address %s", ctr.ID(), ctr.requestedMAC.String())
			}
		}
	}

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

	// teardownCNI will clean the requested IP and MAC so we need to set them again
	ctr.requestedIP = requestedIP
	ctr.requestedMAC = requestedMAC
	return r.configureNetNS(ctr, ctr.state.NetNS)
}

func getContainerNetIO(ctr *Container) (*netlink.LinkStatistics, error) {
	var netStats *netlink.LinkStatistics
	// With slirp4netns, we can't collect statistics at present.
	// For now, we allow stats to at least run by returning nil
	if rootless.IsRootless() || ctr.config.NetMode.IsSlirp4netns() {
		return netStats, nil
	}
	netNSPath, netPathErr := getContainerNetNS(ctr)
	if netPathErr != nil {
		return nil, netPathErr
	}
	if netNSPath == "" {
		// If netNSPath is empty, it was set as none, and no netNS was set up
		// this is a valid state and thus return no error, nor any statistics
		return nil, nil
	}
	err := ns.WithNetNSPath(netNSPath, func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(ocicni.DefaultInterfaceName)
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
		// Have to sync to ensure that state is populated
		if err := netNsCtr.syncContainer(); err != nil {
			return nil, err
		}
		logrus.Debugf("Container %s shares network namespace, retrieving network info of container %s", c.ID(), c.config.NetNsCtr)

		return netNsCtr.getContainerNetworkInfo()
	}

	settings := new(define.InspectNetworkSettings)
	settings.Ports = makeInspectPortBindings(c.config.PortMappings)

	networks, isDefault, err := c.networks()
	if err != nil {
		return nil, err
	}

	// We can't do more if the network is down.
	if c.state.NetNS == nil {
		// We still want to make dummy configurations for each CNI net
		// the container joined.
		if len(networks) > 0 {
			settings.Networks = make(map[string]*define.InspectAdditionalNetwork, len(networks))
			for _, net := range networks {
				cniNet := new(define.InspectAdditionalNetwork)
				cniNet.NetworkID = net
				settings.Networks[net] = cniNet
			}
		}

		return settings, nil
	}

	// Set network namespace path
	settings.SandboxKey = c.state.NetNS.Path()

	// If this is empty, we're probably slirp4netns
	if len(c.state.NetworkStatus) == 0 {
		return settings, nil
	}

	// If we have CNI networks - handle that here
	if len(networks) > 0 {
		if len(networks) != len(c.state.NetworkStatus) {
			return nil, errors.Wrapf(define.ErrInternal, "network inspection mismatch: asked to join %d CNI network(s) %v, but have information on %d network(s)", len(networks), networks, len(c.state.NetworkStatus))
		}

		settings.Networks = make(map[string]*define.InspectAdditionalNetwork)

		// CNI results should be in the same order as the list of
		// networks we pass into CNI.
		for index, name := range networks {
			cniResult := c.state.NetworkStatus[index]
			addedNet := new(define.InspectAdditionalNetwork)
			addedNet.NetworkID = name

			basicConfig, err := resultToBasicNetworkConfig(cniResult)
			if err != nil {
				return nil, err
			}

			aliases, err := c.runtime.state.GetNetworkAliases(c, name)
			if err != nil {
				return nil, err
			}
			addedNet.Aliases = aliases

			addedNet.InspectBasicNetworkConfig = basicConfig

			settings.Networks[name] = addedNet
		}

		if !isDefault {
			return settings, nil
		}
	}

	// If not joining networks, we should have at most 1 result
	if len(c.state.NetworkStatus) > 1 {
		return nil, errors.Wrapf(define.ErrInternal, "should have at most 1 CNI result if not joining networks, instead got %d", len(c.state.NetworkStatus))
	}

	if len(c.state.NetworkStatus) == 1 {
		basicConfig, err := resultToBasicNetworkConfig(c.state.NetworkStatus[0])
		if err != nil {
			return nil, err
		}

		settings.InspectBasicNetworkConfig = basicConfig
	}

	return settings, nil
}

// setupNetworkDescriptions adds networks and eth values to the container's
// network descriptions
func (c *Container) setupNetworkDescriptions(networks []string) error {
	// if the map is nil and we have networks
	if c.state.NetInterfaceDescriptions == nil && len(networks) > 0 {
		c.state.NetInterfaceDescriptions = make(ContainerNetworkDescriptions)
	}
	origLen := len(c.state.NetInterfaceDescriptions)
	for _, n := range networks {
		// if the network is not in the map, add it
		if _, exists := c.state.NetInterfaceDescriptions[n]; !exists {
			c.state.NetInterfaceDescriptions.add(n)
		}
	}
	// if the map changed, we need to save the container state
	if origLen != len(c.state.NetInterfaceDescriptions) {
		if err := c.save(); err != nil {
			return err
		}
	}
	return nil
}

// resultToBasicNetworkConfig produces an InspectBasicNetworkConfig from a CNI
// result
func resultToBasicNetworkConfig(result *cnitypes.Result) (define.InspectBasicNetworkConfig, error) {
	config := define.InspectBasicNetworkConfig{}

	for _, ctrIP := range result.IPs {
		size, _ := ctrIP.Address.Mask.Size()
		switch {
		case ctrIP.Version == "4" && config.IPAddress == "":
			config.IPAddress = ctrIP.Address.IP.String()
			config.IPPrefixLen = size
			config.Gateway = ctrIP.Gateway.String()
			if ctrIP.Interface != nil && *ctrIP.Interface < len(result.Interfaces) && *ctrIP.Interface >= 0 {
				config.MacAddress = result.Interfaces[*ctrIP.Interface].Mac
			}
		case ctrIP.Version == "4" && config.IPAddress != "":
			config.SecondaryIPAddresses = append(config.SecondaryIPAddresses, ctrIP.Address.String())
			if ctrIP.Interface != nil && *ctrIP.Interface < len(result.Interfaces) && *ctrIP.Interface >= 0 {
				config.AdditionalMacAddresses = append(config.AdditionalMacAddresses, result.Interfaces[*ctrIP.Interface].Mac)
			}
		case ctrIP.Version == "6" && config.IPAddress == "":
			config.GlobalIPv6Address = ctrIP.Address.IP.String()
			config.GlobalIPv6PrefixLen = size
			config.IPv6Gateway = ctrIP.Gateway.String()
		case ctrIP.Version == "6" && config.IPAddress != "":
			config.SecondaryIPv6Addresses = append(config.SecondaryIPv6Addresses, ctrIP.Address.String())
		default:
			return config, errors.Wrapf(define.ErrInternal, "unrecognized IP version %q", ctrIP.Version)
		}
	}

	return config, nil
}

// This is a horrible hack, necessary because CNI does not properly clean up
// after itself on an unclean reboot. Return what we're pretty sure is the path
// to CNI's internal files (it's not really exposed to us).
func getCNINetworksDir() (string, error) {
	return "/var/lib/cni/networks", nil
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

	networks, err := c.networksByNameIndex()
	if err != nil {
		return err
	}

	// check if network exists and if the input is a ID we get the name
	// ocicni only uses names so it is important that we only use the name
	netName, err = network.NormalizeName(c.runtime.config, netName)
	if err != nil {
		return err
	}

	index, nameExists := networks[netName]
	if !nameExists && len(networks) > 0 {
		return errors.Errorf("container %s is not connected to network %s", nameOrID, netName)
	}

	c.lock.Lock()
	defer c.lock.Unlock()
	if err := c.syncContainer(); err != nil {
		return err
	}

	if err := c.runtime.state.NetworkDisconnect(c, netName); err != nil {
		return err
	}

	c.newNetworkEvent(events.NetworkDisconnect, netName)
	if c.state.State != define.ContainerStateRunning {
		return nil
	}

	if c.state.NetNS == nil {
		return errors.Wrapf(define.ErrNoNetwork, "unable to disconnect %s from %s", nameOrID, netName)
	}

	podConfig := c.runtime.getPodNetwork(c.ID(), c.Name(), c.state.NetNS.Path(), []string{netName}, c.config.PortMappings, nil, nil, c.state.NetInterfaceDescriptions)
	if err := c.runtime.teardownOCICNIPod(podConfig); err != nil {
		return err
	}

	// update network status if container is not running
	networkStatus := c.state.NetworkStatus
	// clip out the index of the network
	tmpNetworkStatus := make([]*cnitypes.Result, 0, len(networkStatus)-1)
	for k, v := range networkStatus {
		if index != k {
			tmpNetworkStatus = append(tmpNetworkStatus, v)
		}
	}
	c.state.NetworkStatus = tmpNetworkStatus
	return c.save()
}

// ConnectNetwork connects a container to a given network
func (c *Container) NetworkConnect(nameOrID, netName string, aliases []string) error {
	// only the bridge mode supports cni networks
	if err := isBridgeNetMode(c.config.NetMode); err != nil {
		return err
	}

	networks, err := c.networksByNameIndex()
	if err != nil {
		return err
	}

	// check if network exists and if the input is a ID we get the name
	// ocicni only uses names so it is important that we only use the name
	netName, err = network.NormalizeName(c.runtime.config, netName)
	if err != nil {
		return err
	}

	c.lock.Lock()
	defer c.lock.Unlock()
	if err := c.syncContainer(); err != nil {
		return err
	}

	if err := c.runtime.state.NetworkConnect(c, netName, aliases); err != nil {
		return err
	}
	c.newNetworkEvent(events.NetworkConnect, netName)
	if c.state.State != define.ContainerStateRunning {
		return nil
	}
	if c.state.NetNS == nil {
		return errors.Wrapf(define.ErrNoNetwork, "unable to connect %s to %s", nameOrID, netName)
	}

	ctrNetworks, _, err := c.networks()
	if err != nil {
		return err
	}
	// Update network descriptions
	if err := c.setupNetworkDescriptions(ctrNetworks); err != nil {
		return err
	}
	podConfig := c.runtime.getPodNetwork(c.ID(), c.Name(), c.state.NetNS.Path(), []string{netName}, c.config.PortMappings, nil, nil, c.state.NetInterfaceDescriptions)
	podConfig.Aliases = make(map[string][]string, 1)
	podConfig.Aliases[netName] = aliases
	results, err := c.runtime.setUpOCICNIPod(podConfig)
	if err != nil {
		return err
	}
	if len(results) != 1 {
		return errors.New("when adding aliases, results must be of length 1")
	}

	networkResults := make([]*cnitypes.Result, 0)
	for _, r := range results {
		resultCurrent, err := cnitypes.GetResult(r.Result)
		if err != nil {
			return errors.Wrapf(err, "error parsing CNI plugin result %q: %v", r.Result, err)
		}
		networkResults = append(networkResults, resultCurrent)
	}

	// update network status
	networkStatus := c.state.NetworkStatus
	// if len is one and we confirmed earlier that the container is in
	// fact connected to the network, then just return an empty slice
	if len(networkStatus) == 0 {
		c.state.NetworkStatus = append(c.state.NetworkStatus, networkResults...)
	} else {
		// build a list of network names so we can sort and
		// get the new name's index
		var networkNames []string
		for name := range networks {
			networkNames = append(networkNames, name)
		}
		networkNames = append(networkNames, netName)
		// sort
		sort.Strings(networkNames)
		// get index of new network name
		index := sort.SearchStrings(networkNames, netName)
		// Append a zero value to to the slice
		networkStatus = append(networkStatus, &cnitypes.Result{})
		// populate network status
		copy(networkStatus[index+1:], networkStatus[index:])
		networkStatus[index] = networkResults[0]
		c.state.NetworkStatus = networkStatus
	}
	return c.save()
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
func (r *Runtime) ConnectContainerToNetwork(nameOrID, netName string, aliases []string) error {
	ctr, err := r.LookupContainer(nameOrID)
	if err != nil {
		return err
	}
	return ctr.NetworkConnect(nameOrID, netName, aliases)
}
