//go:build freebsd
// +build freebsd

package libpod

import (
	"crypto/rand"
	jdec "encoding/json"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/containers/buildah/pkg/jail"
	"github.com/containers/common/libnetwork/types"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/libpod/events"
	"github.com/containers/podman/v4/pkg/namespaces"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/containers/storage/pkg/lockfile"
	"github.com/sirupsen/logrus"
)

type Netstat struct {
	Statistics NetstatInterface `json:"statistics"`
}

type NetstatInterface struct {
	Interface []NetstatAddress `json:"interface"`
}

type NetstatAddress struct {
	Name    string `json:"name"`
	Flags   string `json:"flags"`
	Mtu     int    `json:"mtu"`
	Network string `json:"network"`
	Address string `json:"address"`

	ReceivedPackets uint64 `json:"received-packets"`
	ReceivedBytes   uint64 `json:"received-bytes"`
	ReceivedErrors  uint64 `json:"received-errors"`

	SentPackets uint64 `json:"sent-packets"`
	SentBytes   uint64 `json:"sent-bytes"`
	SentErrors  uint64 `json:"send-errors"`

	DroppedPackets uint64 `json:"dropped-packets"`

	Collisions uint64 `json:"collisions"`
}

// copied from github.com/vishvanada/netlink which does not build on freebsd
type LinkStatistics64 struct {
	RxPackets         uint64
	TxPackets         uint64
	RxBytes           uint64
	TxBytes           uint64
	RxErrors          uint64
	TxErrors          uint64
	RxDropped         uint64
	TxDropped         uint64
	Multicast         uint64
	Collisions        uint64
	RxLengthErrors    uint64
	RxOverErrors      uint64
	RxCrcErrors       uint64
	RxFrameErrors     uint64
	RxFifoErrors      uint64
	RxMissedErrors    uint64
	TxAbortedErrors   uint64
	TxCarrierErrors   uint64
	TxFifoErrors      uint64
	TxHeartbeatErrors uint64
	TxWindowErrors    uint64
	RxCompressed      uint64
	TxCompressed      uint64
}

type RootlessNetNS struct {
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
	return errors.New("not supported on freebsd")
}

// Cleanup the rootless network namespace if needed.
// It checks if we have running containers with the bridge network mode.
// Cleanup() expects that r.Lock is locked
func (r *RootlessNetNS) Cleanup(runtime *Runtime) error {
	return errors.New("not supported on freebsd")
}

// GetRootlessNetNs returns the rootless netns object. If create is set to true
// the rootless network namespace will be created if it does not exists already.
// If called as root it returns always nil.
// On success the returned RootlessCNI lock is locked and must be unlocked by the caller.
func (r *Runtime) GetRootlessNetNs(new bool) (*RootlessNetNS, error) {
	return nil, nil
}

func GetSlirp4netnsIP(subnet *net.IPNet) (*net.IP, error) {
	return nil, errors.New("not implemented GetSlirp4netnsIP")
}

// While there is code in container_internal.go which calls this, in
// my testing network creation always seems to go through createNetNS.
func (r *Runtime) setupNetNS(ctr *Container) error {
	return errors.New("not implemented (*Runtime) setupNetNS")
}

// Create and configure a new network namespace for a container
func (r *Runtime) configureNetNS(ctr *Container, ctrNS *jailNetNS) (status map[string]types.StatusBlock, rerr error) {
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
	netStatus, err := r.setUpNetwork(ctrNS.Name, netOpts)
	if err != nil {
		return nil, err
	}

	return netStatus, err
}

// Create and configure a new network namespace for a container
func (r *Runtime) createNetNS(ctr *Container) (n *jailNetNS, q map[string]types.StatusBlock, retErr error) {
	b := make([]byte, 16)
	_, err := rand.Reader.Read(b)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate random vnet name: %v", err)
	}
	ctrNS := &jailNetNS{Name: fmt.Sprintf("vnet-%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])}

	jconf := jail.NewConfig()
	jconf.Set("name", ctrNS.Name)
	jconf.Set("vnet", jail.NEW)
	jconf.Set("children.max", 1)
	jconf.Set("persist", true)
	jconf.Set("enforce_statfs", 0)
	jconf.Set("devfs_ruleset", 4)
	jconf.Set("allow.raw_sockets", true)
	jconf.Set("allow.chflags", true)
	jconf.Set("securelevel", -1)
	if _, err := jail.Create(jconf); err != nil {
		logrus.Debugf("Failed to create vnet jail %s for container %s", ctrNS.Name, ctr.ID())
	}

	logrus.Debugf("Created vnet jail %s for container %s", ctrNS.Name, ctr.ID())

	var networkStatus map[string]types.StatusBlock
	networkStatus, err = r.configureNetNS(ctr, ctrNS)
	return ctrNS, networkStatus, err
}

// Tear down a container's network configuration and joins the
// rootless net ns as rootless user
func (r *Runtime) teardownNetwork(ns string, opts types.NetworkOptions) error {
	if err := r.network.Teardown(ns, types.TeardownOptions{NetworkOptions: opts}); err != nil {
		return fmt.Errorf("tearing down network namespace configuration for container %s: %w", opts.ContainerID, err)
	}
	return nil
}

// Tear down a container's CNI network configuration, but do not tear down the
// namespace itself.
func (r *Runtime) teardownCNI(ctr *Container) error {
	if ctr.state.NetNS == nil {
		// The container has no network namespace, we're set
		return nil
	}

	logrus.Debugf("Tearing down network namespace at %s for container %s", ctr.state.NetNS.Name, ctr.ID())

	networks, err := ctr.networks()
	if err != nil {
		return err
	}

	if !ctr.config.NetMode.IsSlirp4netns() && len(networks) > 0 {
		netOpts := ctr.getNetworkOptions(networks)
		return r.teardownNetwork(ctr.state.NetNS.Name, netOpts)
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

	if ctr.state.NetNS != nil {
		// Rather than destroying the jail immediately, reset the
		// persist flag so that it will live until the container is
		// done.
		netjail, err := jail.FindByName(ctr.state.NetNS.Name)
		if err != nil {
			return fmt.Errorf("finding network jail %s: %w", ctr.state.NetNS.Name, err)
		}
		jconf := jail.NewConfig()
		jconf.Set("persist", false)
		if err := netjail.Set(jconf); err != nil {
			return fmt.Errorf("releasing network jail %s: %w", ctr.state.NetNS.Name, err)
		}

		ctr.state.NetNS = nil
	}

	return nil
}

// isBridgeNetMode checks if the given network mode is bridge.
// It returns nil when it is set to bridge and an error otherwise.
func isBridgeNetMode(n namespaces.NetworkMode) error {
	if !n.IsBridge() {
		return fmt.Errorf("%q is not supported: %w", n, define.ErrNetworkModeInvalid)
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
		return nil, fmt.Errorf("container %s network is not configured, refusing to reload: %w", ctr.ID(), define.ErrCtrStateInvalid)
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

func getContainerNetIO(ctr *Container) (*LinkStatistics64, error) {
	if ctr.state.NetNS == nil {
		// If NetNS is nil, it was set as none, and no netNS
		// was set up this is a valid state and thus return no
		// error, nor any statistics
		return nil, nil
	}

	// FIXME get the interface from the container netstatus
	cmd := exec.Command("jexec", ctr.state.NetNS.Name, "netstat", "-bI", "eth0", "--libxo", "json")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	stats := Netstat{}
	if err := jdec.Unmarshal(out, &stats); err != nil {
		return nil, err
	}

	// Find the link stats
	for _, ifaddr := range stats.Statistics.Interface {
		if ifaddr.Mtu > 0 {
			return &LinkStatistics64{
				RxPackets:  ifaddr.ReceivedPackets,
				TxPackets:  ifaddr.SentPackets,
				RxBytes:    ifaddr.ReceivedBytes,
				TxBytes:    ifaddr.SentBytes,
				RxErrors:   ifaddr.ReceivedErrors,
				TxErrors:   ifaddr.SentErrors,
				RxDropped:  ifaddr.DroppedPackets,
				Collisions: ifaddr.Collisions,
			}, nil
		}
	}

	return &LinkStatistics64{}, nil
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

	netStatus := c.getNetworkStatus()
	// If this is empty, we're probably slirp4netns
	if len(netStatus) == 0 {
		return settings, nil
	}

	// If we have networks - handle that here
	if len(networks) > 0 {
		if len(networks) != len(netStatus) {
			return nil, fmt.Errorf("network inspection mismatch: asked to join %d network(s) %v, but have information on %d network(s): %w", len(networks), networks, len(netStatus), define.ErrInternal)
		}

		settings.Networks = make(map[string]*define.InspectAdditionalNetwork)

		for name, opts := range networks {
			result := netStatus[name]
			addedNet := new(define.InspectAdditionalNetwork)
			addedNet.NetworkID = name

			basicConfig := resultToBasicNetworkConfig(result)
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
		return nil, fmt.Errorf("should have at most 1 network status result if not joining networks, instead got %d: %w", len(netStatus), define.ErrInternal)
	}

	if len(netStatus) == 1 {
		for _, status := range netStatus {
			basicConfig := resultToBasicNetworkConfig(status)
			settings.InspectBasicNetworkConfig = basicConfig
		}
	}
	return settings, nil
}

// resultToBasicNetworkConfig produces an InspectBasicNetworkConfig from a CNI
// result
func resultToBasicNetworkConfig(result types.StatusBlock) define.InspectBasicNetworkConfig {
	config := define.InspectBasicNetworkConfig{}
	interfaceNames := make([]string, 0, len(result.Interfaces))
	for interfaceName := range result.Interfaces {
		interfaceNames = append(interfaceNames, interfaceName)
	}
	// ensure consistent inspect results by sorting
	sort.Strings(interfaceNames)
	for _, interfaceName := range interfaceNames {
		netInt := result.Interfaces[interfaceName]
		for _, netAddress := range netInt.Subnets {
			size, _ := netAddress.IPNet.Mask.Size()
			if netAddress.IPNet.IP.To4() != nil {
				// ipv4
				if config.IPAddress == "" {
					config.IPAddress = netAddress.IPNet.IP.String()
					config.IPPrefixLen = size
					config.Gateway = netAddress.Gateway.String()
				} else {
					config.SecondaryIPAddresses = append(config.SecondaryIPAddresses, define.Address{Addr: netAddress.IPNet.IP.String(), PrefixLength: size})
				}
			} else {
				// ipv6
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
	return config
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
		return fmt.Errorf("container %s is not connected to network %s", nameOrID, netName)
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

	opts := types.NetworkOptions{
		ContainerID:   c.config.ID,
		ContainerName: getCNIPodName(c),
	}
	opts.PortMappings = c.convertPortMappings()
	opts.Networks = map[string]types.PerNetworkOptions{
		netName: networks[netName],
	}

	// update network status if container is running
	oldStatus, statusExist := networkStatus[netName]
	delete(networkStatus, netName)
	c.state.NetworkStatus = networkStatus
	err = c.save()
	if err != nil {
		return err
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

	opts := types.NetworkOptions{
		ContainerID:   c.config.ID,
		ContainerName: getCNIPodName(c),
	}
	opts.PortMappings = c.convertPortMappings()
	opts.Networks = map[string]types.PerNetworkOptions{
		netName: netOpts,
	}

	/*
		results, err := c.runtime.setUpNetwork(c.state.NetNS.Path(), opts)
		if err != nil {
			return err
		}
		if len(results) != 1 {
			return errors.New("when adding aliases, results must be of length 1")
		}
	*/
	var results map[string]types.StatusBlock

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

func (c *Container) setupRootlessNetwork() error {
	return nil
}
