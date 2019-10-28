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
	"strconv"
	"strings"
	"syscall"
	"time"

	cnitypes "github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containers/libpod/pkg/errorhandling"
	"github.com/containers/libpod/pkg/netns"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// Get an OCICNI network config
func (r *Runtime) getPodNetwork(id, name, nsPath string, networks []string, ports []ocicni.PortMapping, staticIP net.IP) ocicni.PodNetwork {
	defaultNetwork := r.netPlugin.GetDefaultNetworkName()
	network := ocicni.PodNetwork{
		Name:      name,
		Namespace: name, // TODO is there something else we should put here? We don't know about Kube namespaces
		ID:        id,
		NetNS:     nsPath,
		Networks:  networks,
		RuntimeConfig: map[string]ocicni.RuntimeConfig{
			defaultNetwork: {PortMappings: ports},
		},
	}

	if staticIP != nil {
		network.Networks = []string{defaultNetwork}
		network.RuntimeConfig = map[string]ocicni.RuntimeConfig{
			defaultNetwork: {IP: staticIP.String(), PortMappings: ports},
		}
	}

	return network
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

	podNetwork := r.getPodNetwork(ctr.ID(), ctr.Name(), ctrNS.Path(), ctr.config.Networks, ctr.config.PortMappings, requestedIP)

	results, err := r.netPlugin.SetUpPod(podNetwork)
	if err != nil {
		return nil, errors.Wrapf(err, "error configuring network namespace for container %s", ctr.ID())
	}
	defer func() {
		if err != nil {
			if err2 := r.netPlugin.TearDownPod(podNetwork); err2 != nil {
				logrus.Errorf("Error tearing down partially created network namespace for container %s: %v", ctr.ID(), err2)
			}
		}
	}()

	networkStatus := make([]*cnitypes.Result, 0)
	for idx, r := range results {
		logrus.Debugf("[%d] CNI result: %v", idx, r.String())
		resultCurrent, err := cnitypes.GetResult(r)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing CNI plugin result %q: %v", r.String(), err)
		}
		networkStatus = append(networkStatus, resultCurrent)
	}

	return networkStatus, nil
}

// Create and configure a new network namespace for a container
func (r *Runtime) createNetNS(ctr *Container) (n ns.NetNS, q []*cnitypes.Result, err error) {
	ctrNS, err := netns.NewNS()
	if err != nil {
		return nil, nil, errors.Wrapf(err, "error creating network namespace for container %s", ctr.ID())
	}
	defer func() {
		if err != nil {
			if err2 := netns.UnmountNS(ctrNS); err2 != nil {
				logrus.Errorf("Error unmounting partially created network namespace for container %s: %v", ctr.ID(), err2)
			}
			if err2 := ctrNS.Close(); err2 != nil {
				logrus.Errorf("Error closing partially created network namespace for container %s: %v", ctr.ID(), err2)
			}
		}
	}()

	logrus.Debugf("Made network namespace at %s for container %s", ctrNS.Path(), ctr.ID())

	networkStatus := []*cnitypes.Result{}
	if !rootless.IsRootless() {
		networkStatus, err = r.configureNetNS(ctr, ctrNS)
	}
	return ctrNS, networkStatus, err
}

type slirp4netnsCmdArg struct {
	Proto     string `json:"proto,omitempty"`
	HostAddr  string `json:"host_addr"`
	HostPort  int32  `json:"host_port"`
	GuestAddr string `json:"guest_addr"`
	GuestPort int32  `json:"guest_port"`
}

type slirp4netnsCmd struct {
	Execute string            `json:"execute"`
	Args    slirp4netnsCmdArg `json:"arguments"`
}

func checkSlirpFlags(path string) (bool, bool, bool, error) {
	cmd := exec.Command(path, "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, false, false, errors.Wrapf(err, "slirp4netns %q", out)
	}
	return strings.Contains(string(out), "--disable-host-loopback"), strings.Contains(string(out), "--mtu"), strings.Contains(string(out), "--enable-sandbox"), nil
}

// Configure the network namespace for a rootless container
func (r *Runtime) setupRootlessNetNS(ctr *Container) (err error) {
	path := r.config.NetworkCmdPath

	if path == "" {
		var err error
		path, err = exec.LookPath("slirp4netns")
		if err != nil {
			logrus.Errorf("could not find slirp4netns, the network namespace won't be configured: %v", err)
			return nil
		}
	}

	syncR, syncW, err := os.Pipe()
	if err != nil {
		return errors.Wrapf(err, "failed to open pipe")
	}
	defer errorhandling.CloseQuiet(syncR)
	defer errorhandling.CloseQuiet(syncW)

	havePortMapping := len(ctr.Config().PortMappings) > 0
	apiSocket := filepath.Join(ctr.runtime.config.TmpDir, fmt.Sprintf("%s.net", ctr.config.ID))
	logPath := filepath.Join(ctr.runtime.config.TmpDir, fmt.Sprintf("slirp4netns-%s.log", ctr.config.ID))

	cmdArgs := []string{}
	if havePortMapping {
		cmdArgs = append(cmdArgs, "--api-socket", apiSocket)
	}
	dhp, mtu, sandbox, err := checkSlirpFlags(path)
	if err != nil {
		return errors.Wrapf(err, "error checking slirp4netns binary %s: %q", path, err)
	}
	if dhp {
		cmdArgs = append(cmdArgs, "--disable-host-loopback")
	}
	if mtu {
		cmdArgs = append(cmdArgs, "--mtu", "65520")
	}
	if sandbox {
		cmdArgs = append(cmdArgs, "--enable-sandbox")
	}

	// the slirp4netns arguments being passed are describes as follows:
	// from the slirp4netns documentation: https://github.com/rootless-containers/slirp4netns
	// -c, --configure Brings up the tap interface
	// -e, --exit-fd=FD specify the FD for terminating slirp4netns
	// -r, --ready-fd=FD specify the FD to write to when the initialization steps are finished
	cmdArgs = append(cmdArgs, "-c", "-e", "3", "-r", "4")
	if !ctr.config.PostConfigureNetNS {
		ctr.rootlessSlirpSyncR, ctr.rootlessSlirpSyncW, err = os.Pipe()
		if err != nil {
			return errors.Wrapf(err, "failed to create rootless network sync pipe")
		}
		cmdArgs = append(cmdArgs, "--netns-type=path", ctr.state.NetNS.Path(), "tap0")
	} else {
		defer errorhandling.CloseQuiet(ctr.rootlessSlirpSyncR)
		defer errorhandling.CloseQuiet(ctr.rootlessSlirpSyncW)
		cmdArgs = append(cmdArgs, fmt.Sprintf("%d", ctr.state.PID), "tap0")
	}

	cmd := exec.Command(path, cmdArgs...)
	logrus.Debugf("slirp4netns command: %s", strings.Join(cmd.Args, " "))
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// workaround for https://github.com/rootless-containers/slirp4netns/pull/153
	if sandbox {
		cmd.SysProcAttr.Cloneflags = syscall.CLONE_NEWNS
		cmd.SysProcAttr.Unshareflags = syscall.CLONE_NEWNS
	}

	// Leak one end of the pipe in slirp4netns, the other will be sent to conmon
	cmd.ExtraFiles = append(cmd.ExtraFiles, ctr.rootlessSlirpSyncR, syncW)

	logFile, err := os.Create(logPath)
	if err != nil {
		return errors.Wrapf(err, "failed to open slirp4netns log file %s", logPath)
	}
	defer logFile.Close()
	// Unlink immediately the file so we won't need to worry about cleaning it up later.
	// It is still accessible through the open fd logFile.
	if err := os.Remove(logPath); err != nil {
		return errors.Wrapf(err, "delete file %s", logPath)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return errors.Wrapf(err, "failed to start slirp4netns process")
	}
	defer func() {
		if err := cmd.Process.Release(); err != nil {
			logrus.Errorf("unable to release comman process: %q", err)
		}
	}()

	b := make([]byte, 16)
	for {
		if err := syncR.SetDeadline(time.Now().Add(1 * time.Second)); err != nil {
			return errors.Wrapf(err, "error setting slirp4netns pipe timeout")
		}
		if _, err := syncR.Read(b); err == nil {
			break
		} else {
			if os.IsTimeout(err) {
				// Check if the process is still running.
				var status syscall.WaitStatus
				pid, err := syscall.Wait4(cmd.Process.Pid, &status, syscall.WNOHANG, nil)
				if err != nil {
					return errors.Wrapf(err, "failed to read slirp4netns process status")
				}
				if pid != cmd.Process.Pid {
					continue
				}
				if status.Exited() {
					// Seek at the beginning of the file and read all its content
					if _, err := logFile.Seek(0, 0); err != nil {
						logrus.Errorf("could not seek log file: %q", err)
					}
					logContent, err := ioutil.ReadAll(logFile)
					if err != nil {
						return errors.Wrapf(err, "slirp4netns failed")
					}
					return errors.Errorf("slirp4netns failed: %q", logContent)
				}
				if status.Signaled() {
					return errors.New("slirp4netns killed by signal")
				}
				continue
			}
			return errors.Wrapf(err, "failed to read from slirp4netns sync pipe")
		}
	}

	if havePortMapping {
		const pidWaitTimeout = 60 * time.Second
		chWait := make(chan error)
		go func() {
			interval := 25 * time.Millisecond
			for i := time.Duration(0); i < pidWaitTimeout; i += interval {
				// Check if the process is still running.
				var status syscall.WaitStatus
				pid, err := syscall.Wait4(cmd.Process.Pid, &status, syscall.WNOHANG, nil)
				if err != nil {
					break
				}
				if pid != cmd.Process.Pid {
					continue
				}
				if status.Exited() || status.Signaled() {
					chWait <- fmt.Errorf("slirp4netns exited with status %d", status.ExitStatus())
				}
				time.Sleep(interval)
			}
		}()
		defer close(chWait)

		// wait that API socket file appears before trying to use it.
		if _, err := WaitForFile(apiSocket, chWait, pidWaitTimeout*time.Millisecond); err != nil {
			return errors.Wrapf(err, "waiting for slirp4nets to create the api socket file %s", apiSocket)
		}

		// for each port we want to add we need to open a connection to the slirp4netns control socket
		// and send the add_hostfwd command.
		for _, i := range ctr.config.PortMappings {
			conn, err := net.Dial("unix", apiSocket)
			if err != nil {
				return errors.Wrapf(err, "cannot open connection to %s", apiSocket)
			}
			defer func() {
				if err := conn.Close(); err != nil {
					logrus.Errorf("unable to close connection: %q", err)
				}
			}()
			hostIP := i.HostIP
			if hostIP == "" {
				hostIP = "0.0.0.0"
			}
			cmd := slirp4netnsCmd{
				Execute: "add_hostfwd",
				Args: slirp4netnsCmdArg{
					Proto:     i.Protocol,
					HostAddr:  hostIP,
					HostPort:  i.HostPort,
					GuestPort: i.ContainerPort,
				},
			}
			// create the JSON payload and send it.  Mark the end of request shutting down writes
			// to the socket, as requested by slirp4netns.
			data, err := json.Marshal(&cmd)
			if err != nil {
				return errors.Wrapf(err, "cannot marshal JSON for slirp4netns")
			}
			if _, err := conn.Write([]byte(fmt.Sprintf("%s\n", data))); err != nil {
				return errors.Wrapf(err, "cannot write to control socket %s", apiSocket)
			}
			if err := conn.(*net.UnixConn).CloseWrite(); err != nil {
				return errors.Wrapf(err, "cannot shutdown the socket %s", apiSocket)
			}
			buf := make([]byte, 2048)
			readLength, err := conn.Read(buf)
			if err != nil {
				return errors.Wrapf(err, "cannot read from control socket %s", apiSocket)
			}
			// if there is no 'error' key in the received JSON data, then the operation was
			// successful.
			var y map[string]interface{}
			if err := json.Unmarshal(buf[0:readLength], &y); err != nil {
				return errors.Wrapf(err, "error parsing error status from slirp4netns")
			}
			if e, found := y["error"]; found {
				return errors.Errorf("error from slirp4netns while setting up port redirection: %v", e)
			}
		}
	}
	return nil
}

// Configure the network namespace using the container process
func (r *Runtime) setupNetNS(ctr *Container) (err error) {
	nsProcess := fmt.Sprintf("/proc/%d/ns/net", ctr.state.PID)

	b := make([]byte, 16)

	if _, err := rand.Reader.Read(b); err != nil {
		return errors.Wrapf(err, "failed to generate random netns name")
	}

	nsPath := fmt.Sprintf("/var/run/netns/cni-%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])

	if err := os.MkdirAll(filepath.Dir(nsPath), 0711); err != nil {
		return errors.Wrapf(err, "cannot create %s", filepath.Dir(nsPath))
	}

	mountPointFd, err := os.Create(nsPath)
	if err != nil {
		return errors.Wrapf(err, "cannot open %s", nsPath)
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

// Tear down a network namespace, undoing all state associated with it.
func (r *Runtime) teardownNetNS(ctr *Container) error {
	if ctr.state.NetNS == nil {
		// The container has no network namespace, we're set
		return nil
	}

	logrus.Debugf("Tearing down network namespace at %s for container %s", ctr.state.NetNS.Path(), ctr.ID())

	// rootless containers do not use the CNI plugin
	if !rootless.IsRootless() {
		var requestedIP net.IP
		if ctr.requestedIP != nil {
			requestedIP = ctr.requestedIP
			// cancel request for a specific IP in case the container is reused later
			ctr.requestedIP = nil
		} else {
			requestedIP = ctr.config.StaticIP
		}

		podNetwork := r.getPodNetwork(ctr.ID(), ctr.Name(), ctr.state.NetNS.Path(), ctr.config.Networks, ctr.config.PortMappings, requestedIP)

		if err := r.netPlugin.TearDownPod(podNetwork); err != nil {
			return errors.Wrapf(err, "error tearing down CNI namespace configuration for container %s", ctr.ID())
		}
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
		return c.state.NetNS.Path(), nil
	}
	return "", nil
}

func getContainerNetIO(ctr *Container) (*netlink.LinkStatistics, error) {
	var netStats *netlink.LinkStatistics
	// rootless v2 cannot seem to resolve its network connection to
	// collect statistics.  For now, we allow stats to at least run
	// by returning nil
	if rootless.IsRootless() {
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

func (c *Container) getContainerNetworkInfo(data *InspectContainerData) *InspectContainerData {
	if c.state.NetNS != nil && len(c.state.NetworkStatus) > 0 {
		// Report network settings from the first pod network
		result := c.state.NetworkStatus[0]
		// Go through our IP addresses
		for _, ctrIP := range result.IPs {
			ipWithMask := ctrIP.Address.String()
			splitIP := strings.Split(ipWithMask, "/")
			mask, _ := strconv.Atoi(splitIP[1])
			if ctrIP.Version == "4" {
				data.NetworkSettings.IPAddress = splitIP[0]
				data.NetworkSettings.IPPrefixLen = mask
				data.NetworkSettings.Gateway = ctrIP.Gateway.String()
			} else {
				data.NetworkSettings.GlobalIPv6Address = splitIP[0]
				data.NetworkSettings.GlobalIPv6PrefixLen = mask
				data.NetworkSettings.IPv6Gateway = ctrIP.Gateway.String()
			}
		}

		// Set network namespace path
		data.NetworkSettings.SandboxKey = c.state.NetNS.Path()

		// Set MAC address of interface linked with network namespace path
		for _, i := range result.Interfaces {
			if i.Sandbox == data.NetworkSettings.SandboxKey {
				data.NetworkSettings.MacAddress = i.Mac
			}
		}
	}
	return data
}
