// +build linux

package libpod

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	cnitypes "github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/libpod/events"
	"github.com/containers/podman/v2/libpod/network"
	"github.com/containers/podman/v2/pkg/errorhandling"
	"github.com/containers/podman/v2/pkg/netns"
	"github.com/containers/podman/v2/pkg/rootless"
	"github.com/containers/podman/v2/pkg/rootlessport"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
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
	if !rootless.IsRootless() && !ctr.config.NetMode.IsSlirp4netns() {
		networkStatus, err = r.configureNetNS(ctr, ctrNS)
	}
	return ctrNS, networkStatus, err
}

type slirpFeatures struct {
	HasDisableHostLoopback bool
	HasMTU                 bool
	HasEnableSandbox       bool
	HasEnableSeccomp       bool
	HasCIDR                bool
	HasOutboundAddr        bool
	HasIPv6                bool
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

func checkSlirpFlags(path string) (*slirpFeatures, error) {
	cmd := exec.Command(path, "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, errors.Wrapf(err, "slirp4netns %q", out)
	}
	return &slirpFeatures{
		HasDisableHostLoopback: strings.Contains(string(out), "--disable-host-loopback"),
		HasMTU:                 strings.Contains(string(out), "--mtu"),
		HasEnableSandbox:       strings.Contains(string(out), "--enable-sandbox"),
		HasEnableSeccomp:       strings.Contains(string(out), "--enable-seccomp"),
		HasCIDR:                strings.Contains(string(out), "--cidr"),
		HasOutboundAddr:        strings.Contains(string(out), "--outbound-addr"),
		HasIPv6:                strings.Contains(string(out), "--enable-ipv6"),
	}, nil
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
	if len(networks) > 0 {
		// set up port forwarder for CNI-in-slirp4netns
		netnsPath := ctr.state.NetNS.Path()
		// TODO: support slirp4netns port forwarder as well
		return r.setupRootlessPortMappingViaRLK(ctr, netnsPath)
	}
	return nil
}

// setupSlirp4netns can be called in rootful as well as in rootless
func (r *Runtime) setupSlirp4netns(ctr *Container) error {
	path := r.config.Engine.NetworkCmdPath

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
	logPath := filepath.Join(ctr.runtime.config.Engine.TmpDir, fmt.Sprintf("slirp4netns-%s.log", ctr.config.ID))

	cidr := ""
	isSlirpHostForward := false
	disableHostLoopback := true
	enableIPv6 := false
	outboundAddr := ""
	outboundAddr6 := ""

	if ctr.config.NetworkOptions != nil {
		slirpOptions := ctr.config.NetworkOptions["slirp4netns"]
		for _, o := range slirpOptions {
			parts := strings.SplitN(o, "=", 2)
			if len(parts) < 2 {
				return errors.Errorf("unknown option for slirp4netns: %q", o)
			}
			option, value := parts[0], parts[1]
			switch option {
			case "cidr":
				ipv4, _, err := net.ParseCIDR(value)
				if err != nil || ipv4.To4() == nil {
					return errors.Errorf("invalid cidr %q", value)
				}
				cidr = value
			case "port_handler":
				switch value {
				case "slirp4netns":
					isSlirpHostForward = true
				case "rootlesskit":
					isSlirpHostForward = false
				default:
					return errors.Errorf("unknown port_handler for slirp4netns: %q", value)
				}
			case "allow_host_loopback":
				switch value {
				case "true":
					disableHostLoopback = false
				case "false":
					disableHostLoopback = true
				default:
					return errors.Errorf("invalid value of allow_host_loopback for slirp4netns: %q", value)
				}
			case "enable_ipv6":
				switch value {
				case "true":
					enableIPv6 = true
				case "false":
					enableIPv6 = false
				default:
					return errors.Errorf("invalid value of enable_ipv6 for slirp4netns: %q", value)
				}
			case "outbound_addr":
				ipv4 := net.ParseIP(value)
				if ipv4 == nil || ipv4.To4() == nil {
					_, err := net.InterfaceByName(value)
					if err != nil {
						return errors.Errorf("invalid outbound_addr %q", value)
					}
				}
				outboundAddr = value
			case "outbound_addr6":
				ipv6 := net.ParseIP(value)
				if ipv6 == nil || ipv6.To4() != nil {
					_, err := net.InterfaceByName(value)
					if err != nil {
						return errors.Errorf("invalid outbound_addr6: %q", value)
					}
				}
				outboundAddr6 = value
			default:
				return errors.Errorf("unknown option for slirp4netns: %q", o)
			}
		}
	}

	cmdArgs := []string{}
	slirpFeatures, err := checkSlirpFlags(path)
	if err != nil {
		return errors.Wrapf(err, "error checking slirp4netns binary %s: %q", path, err)
	}
	if disableHostLoopback && slirpFeatures.HasDisableHostLoopback {
		cmdArgs = append(cmdArgs, "--disable-host-loopback")
	}
	if slirpFeatures.HasMTU {
		cmdArgs = append(cmdArgs, "--mtu", "65520")
	}
	if slirpFeatures.HasEnableSandbox {
		cmdArgs = append(cmdArgs, "--enable-sandbox")
	}
	if slirpFeatures.HasEnableSeccomp {
		cmdArgs = append(cmdArgs, "--enable-seccomp")
	}

	if cidr != "" {
		if !slirpFeatures.HasCIDR {
			return errors.Errorf("cidr not supported")
		}
		cmdArgs = append(cmdArgs, fmt.Sprintf("--cidr=%s", cidr))
	}

	if enableIPv6 {
		if !slirpFeatures.HasIPv6 {
			return errors.Errorf("enable_ipv6 not supported")
		}
		cmdArgs = append(cmdArgs, "--enable-ipv6")
	}

	if outboundAddr != "" {
		if !slirpFeatures.HasOutboundAddr {
			return errors.Errorf("outbound_addr not supported")
		}
		cmdArgs = append(cmdArgs, fmt.Sprintf("--outbound-addr=%s", outboundAddr))
	}

	if outboundAddr6 != "" {
		if !slirpFeatures.HasOutboundAddr || !slirpFeatures.HasIPv6 {
			return errors.Errorf("outbound_addr6 not supported")
		}
		if !enableIPv6 {
			return errors.Errorf("enable_ipv6=true is required for outbound_addr6")
		}
		cmdArgs = append(cmdArgs, fmt.Sprintf("--outbound-addr6=%s", outboundAddr6))
	}

	var apiSocket string
	if havePortMapping && isSlirpHostForward {
		apiSocket = filepath.Join(ctr.runtime.config.Engine.TmpDir, fmt.Sprintf("%s.net", ctr.config.ID))
		cmdArgs = append(cmdArgs, "--api-socket", apiSocket)
	}

	// the slirp4netns arguments being passed are describes as follows:
	// from the slirp4netns documentation: https://github.com/rootless-containers/slirp4netns
	// -c, --configure Brings up the tap interface
	// -e, --exit-fd=FD specify the FD for terminating slirp4netns
	// -r, --ready-fd=FD specify the FD to write to when the initialization steps are finished
	cmdArgs = append(cmdArgs, "-c", "-e", "3", "-r", "4")
	netnsPath := ""
	if !ctr.config.PostConfigureNetNS {
		ctr.rootlessSlirpSyncR, ctr.rootlessSlirpSyncW, err = os.Pipe()
		if err != nil {
			return errors.Wrapf(err, "failed to create rootless network sync pipe")
		}
		netnsPath = ctr.state.NetNS.Path()
		cmdArgs = append(cmdArgs, "--netns-type=path", netnsPath, "tap0")
	} else {
		defer errorhandling.CloseQuiet(ctr.rootlessSlirpSyncR)
		defer errorhandling.CloseQuiet(ctr.rootlessSlirpSyncW)
		netnsPath = fmt.Sprintf("/proc/%d/ns/net", ctr.state.PID)
		// we don't use --netns-path here (unavailable for slirp4netns < v0.4)
		cmdArgs = append(cmdArgs, fmt.Sprintf("%d", ctr.state.PID), "tap0")
	}

	cmd := exec.Command(path, cmdArgs...)
	logrus.Debugf("slirp4netns command: %s", strings.Join(cmd.Args, " "))
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// workaround for https://github.com/rootless-containers/slirp4netns/pull/153
	if slirpFeatures.HasEnableSandbox {
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
			logrus.Errorf("unable to release command process: %q", err)
		}
	}()

	if err := waitForSync(syncR, cmd, logFile, 1*time.Second); err != nil {
		return err
	}

	if havePortMapping {
		if isSlirpHostForward {
			return r.setupRootlessPortMappingViaSlirp(ctr, cmd, apiSocket)
		} else {
			return r.setupRootlessPortMappingViaRLK(ctr, netnsPath)
		}
	}
	return nil
}

func waitForSync(syncR *os.File, cmd *exec.Cmd, logFile io.ReadSeeker, timeout time.Duration) error {
	prog := filepath.Base(cmd.Path)
	if len(cmd.Args) > 0 {
		prog = cmd.Args[0]
	}
	b := make([]byte, 16)
	for {
		if err := syncR.SetDeadline(time.Now().Add(timeout)); err != nil {
			return errors.Wrapf(err, "error setting %s pipe timeout", prog)
		}
		// FIXME: return err as soon as proc exits, without waiting for timeout
		if _, err := syncR.Read(b); err == nil {
			break
		} else {
			if os.IsTimeout(err) {
				// Check if the process is still running.
				var status syscall.WaitStatus
				pid, err := syscall.Wait4(cmd.Process.Pid, &status, syscall.WNOHANG, nil)
				if err != nil {
					return errors.Wrapf(err, "failed to read %s process status", prog)
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
						return errors.Wrapf(err, "%s failed", prog)
					}
					return errors.Errorf("%s failed: %q", prog, logContent)
				}
				if status.Signaled() {
					return errors.Errorf("%s killed by signal", prog)
				}
				continue
			}
			return errors.Wrapf(err, "failed to read from %s sync pipe", prog)
		}
	}
	return nil
}

func (r *Runtime) setupRootlessPortMappingViaRLK(ctr *Container, netnsPath string) error {
	syncR, syncW, err := os.Pipe()
	if err != nil {
		return errors.Wrapf(err, "failed to open pipe")
	}
	defer errorhandling.CloseQuiet(syncR)
	defer errorhandling.CloseQuiet(syncW)

	logPath := filepath.Join(ctr.runtime.config.Engine.TmpDir, fmt.Sprintf("rootlessport-%s.log", ctr.config.ID))
	logFile, err := os.Create(logPath)
	if err != nil {
		return errors.Wrapf(err, "failed to open rootlessport log file %s", logPath)
	}
	defer logFile.Close()
	// Unlink immediately the file so we won't need to worry about cleaning it up later.
	// It is still accessible through the open fd logFile.
	if err := os.Remove(logPath); err != nil {
		return errors.Wrapf(err, "delete file %s", logPath)
	}

	if !ctr.config.PostConfigureNetNS {
		ctr.rootlessPortSyncR, ctr.rootlessPortSyncW, err = os.Pipe()
		if err != nil {
			return errors.Wrapf(err, "failed to create rootless port sync pipe")
		}
	}

	cfg := rootlessport.Config{
		Mappings:  ctr.config.PortMappings,
		NetNSPath: netnsPath,
		ExitFD:    3,
		ReadyFD:   4,
		TmpDir:    ctr.runtime.config.Engine.TmpDir,
	}
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	cfgR := bytes.NewReader(cfgJSON)
	var stdout bytes.Buffer
	cmd := exec.Command(fmt.Sprintf("/proc/%d/exe", os.Getpid()))
	cmd.Args = []string{rootlessport.ReexecKey}
	// Leak one end of the pipe in rootlessport process, the other will be sent to conmon

	if ctr.rootlessPortSyncR != nil {
		defer errorhandling.CloseQuiet(ctr.rootlessPortSyncR)
	}

	cmd.ExtraFiles = append(cmd.ExtraFiles, ctr.rootlessPortSyncR, syncW)
	cmd.Stdin = cfgR
	// stdout is for human-readable error, stderr is for debug log
	cmd.Stdout = &stdout
	cmd.Stderr = io.MultiWriter(logFile, &logrusDebugWriter{"rootlessport: "})
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	if err := cmd.Start(); err != nil {
		return errors.Wrapf(err, "failed to start rootlessport process")
	}
	defer func() {
		if err := cmd.Process.Release(); err != nil {
			logrus.Errorf("unable to release rootlessport process: %q", err)
		}
	}()
	if err := waitForSync(syncR, cmd, logFile, 3*time.Second); err != nil {
		stdoutStr := stdout.String()
		if stdoutStr != "" {
			// err contains full debug log and too verbose, so return stdoutStr
			logrus.Debug(err)
			return errors.Errorf("rootlessport " + strings.TrimSuffix(stdoutStr, "\n"))
		}
		return err
	}
	logrus.Debug("rootlessport is ready")
	return nil
}

func (r *Runtime) setupRootlessPortMappingViaSlirp(ctr *Container, cmd *exec.Cmd, apiSocket string) (err error) {
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
	if _, err := WaitForFile(apiSocket, chWait, pidWaitTimeout); err != nil {
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
		apiCmd := slirp4netnsCmd{
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
		data, err := json.Marshal(&apiCmd)
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
	logrus.Debug("slirp4netns port-forwarding setup via add_hostfwd is ready")
	return nil
}

// Configure the network namespace using the container process
func (r *Runtime) setupNetNS(ctr *Container) error {
	nsProcess := fmt.Sprintf("/proc/%d/ns/net", ctr.state.PID)

	b := make([]byte, 16)

	if _, err := rand.Reader.Read(b); err != nil {
		return errors.Wrapf(err, "failed to generate random netns name")
	}

	nsPath := fmt.Sprintf("/var/run/netns/cni-%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])

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

// Tear down a network namespace, undoing all state associated with it.
func (r *Runtime) teardownNetNS(ctr *Container) error {
	if ctr.state.NetNS == nil {
		// The container has no network namespace, we're set
		return nil
	}

	logrus.Debugf("Tearing down network namespace at %s for container %s", ctr.state.NetNS.Path(), ctr.ID())

	networks, _, err := ctr.networks()
	if err != nil {
		return err
	}

	// rootless containers do not use the CNI plugin directly
	if !rootless.IsRootless() && !ctr.config.NetMode.IsSlirp4netns() && len(networks) > 0 {
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

		podNetwork := r.getPodNetwork(ctr.ID(), ctr.Name(), ctr.state.NetNS.Path(), networks, ctr.config.PortMappings, requestedIP, requestedMAC, ContainerNetworkDescriptions{})

		if err := r.netPlugin.TearDownPod(podNetwork); err != nil {
			return errors.Wrapf(err, "error tearing down CNI namespace configuration for container %s", ctr.ID())
		}
	}

	// CNI-in-slirp4netns
	if rootless.IsRootless() && len(networks) != 0 {
		if err := DeallocRootlessCNI(context.Background(), ctr); err != nil {
			return errors.Wrapf(err, "error tearing down CNI-in-slirp4netns for container %s", ctr.ID())
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
		return getContainerNetNS(c)
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
		if len(networks) > 0 && !isDefault {
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
	if len(networks) > 0 && !isDefault {
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

		return settings, nil
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
			if ctrIP.Interface != nil && *ctrIP.Interface < len(result.Interfaces) && *ctrIP.Interface > 0 {
				config.MacAddress = result.Interfaces[*ctrIP.Interface].Mac
			}
		case ctrIP.Version == "4" && config.IPAddress != "":
			config.SecondaryIPAddresses = append(config.SecondaryIPAddresses, ctrIP.Address.String())
			if ctrIP.Interface != nil && *ctrIP.Interface < len(result.Interfaces) && *ctrIP.Interface > 0 {
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
	networks, err := c.networksByNameIndex()
	if err != nil {
		return err
	}

	exists, err := network.Exists(c.runtime.config, netName)
	if err != nil {
		return err
	}
	if !exists {
		return errors.Wrap(define.ErrNoSuchNetwork, netName)
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

	if c.state.State != define.ContainerStateRunning {
		return errors.Wrapf(define.ErrCtrStateInvalid, "cannot disconnect container %s from networks as it is not running", nameOrID)
	}
	if c.state.NetNS == nil {
		return errors.Wrapf(define.ErrNoNetwork, "unable to disconnect %s from %s", nameOrID, netName)
	}
	podConfig := c.runtime.getPodNetwork(c.ID(), c.Name(), c.state.NetNS.Path(), []string{netName}, c.config.PortMappings, nil, nil, c.state.NetInterfaceDescriptions)
	if err := c.runtime.netPlugin.TearDownPod(podConfig); err != nil {
		return err
	}
	if err := c.runtime.state.NetworkDisconnect(c, netName); err != nil {
		return err
	}

	// update network status
	networkStatus := c.state.NetworkStatus
	// clip out the index of the network
	tmpNetworkStatus := make([]*cnitypes.Result, len(networkStatus)-1)
	for k, v := range networkStatus {
		if index != k {
			tmpNetworkStatus = append(tmpNetworkStatus, v)
		}
	}
	c.state.NetworkStatus = tmpNetworkStatus
	c.newNetworkEvent(events.NetworkDisconnect, netName)
	return c.save()
}

// ConnnectNetwork connects a container to a given network
func (c *Container) NetworkConnect(nameOrID, netName string, aliases []string) error {
	networks, err := c.networksByNameIndex()
	if err != nil {
		return err
	}

	exists, err := network.Exists(c.runtime.config, netName)
	if err != nil {
		return err
	}
	if !exists {
		return errors.Wrap(define.ErrNoSuchNetwork, netName)
	}

	c.lock.Lock()
	defer c.lock.Unlock()
	if err := c.syncContainer(); err != nil {
		return err
	}

	if c.state.State != define.ContainerStateRunning {
		return errors.Wrapf(define.ErrCtrStateInvalid, "cannot connect container %s to networks as it is not running", nameOrID)
	}
	if c.state.NetNS == nil {
		return errors.Wrapf(define.ErrNoNetwork, "unable to connect %s to %s", nameOrID, netName)
	}
	if err := c.runtime.state.NetworkConnect(c, netName, aliases); err != nil {
		return err
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
	results, err := c.runtime.netPlugin.SetUpPod(podConfig)
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
	c.newNetworkEvent(events.NetworkConnect, netName)
	return c.save()
}

// DisconnectContainerFromNetwork removes a container from its CNI network
func (r *Runtime) DisconnectContainerFromNetwork(nameOrID, netName string, force bool) error {
	if rootless.IsRootless() {
		return errors.New("network connect is not enabled for rootless containers")
	}
	ctr, err := r.LookupContainer(nameOrID)
	if err != nil {
		return err
	}
	return ctr.NetworkDisconnect(nameOrID, netName, force)
}

// ConnectContainerToNetwork connects a container to a CNI network
func (r *Runtime) ConnectContainerToNetwork(nameOrID, netName string, aliases []string) error {
	if rootless.IsRootless() {
		return errors.New("network disconnect is not enabled for rootless containers")
	}
	ctr, err := r.LookupContainer(nameOrID)
	if err != nil {
		return err
	}
	return ctr.NetworkConnect(nameOrID, netName, aliases)
}
