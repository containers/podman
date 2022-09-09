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

	"github.com/containers/buildah/pkg/jail"
	"github.com/containers/common/libnetwork/types"
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

func (c *Container) joinedNetworkNSPath() string {
	if c.state.NetNS != nil {
		return c.state.NetNS.Name
	} else {
		return ""
	}
}

func (c *Container) inspectJoinedNetworkNS(networkns string) (q types.StatusBlock, retErr error) {
	// TODO: extract interface information from the vnet jail
	return types.StatusBlock{}, nil

}

func (c *Container) reloadRootlessRLKPortMapping() error {
	return errors.New("unsupported (*Container).reloadRootlessRLKPortMapping")
}

func (c *Container) setupRootlessNetwork() error {
	return nil
}
