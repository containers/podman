package specgen

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/cgroups"
	cutil "github.com/containers/common/pkg/util"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/namespaces"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/containers/storage"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
)

type NamespaceMode string

const (
	// Default indicates the spec generator should determine
	// a sane default
	Default NamespaceMode = "default"
	// Host means the namespace is derived from the host
	Host NamespaceMode = "host"
	// Path is the path to a namespace
	Path NamespaceMode = "path"
	// FromContainer means namespace is derived from a
	// different container
	FromContainer NamespaceMode = "container"
	// FromPod indicates the namespace is derived from a pod
	FromPod NamespaceMode = "pod"
	// Private indicates the namespace is private
	Private NamespaceMode = "private"
	// Shareable indicates the namespace is shareable
	Shareable NamespaceMode = "shareable"
	// None indicates the IPC namespace is created without mounting /dev/shm
	None NamespaceMode = "none"
	// NoNetwork indicates no network namespace should
	// be joined.  loopback should still exist.
	// Only used with the network namespace, invalid otherwise.
	NoNetwork NamespaceMode = "none"
	// Bridge indicates that the network backend (CNI/netavark)
	// should be used.
	// Only used with the network namespace, invalid otherwise.
	Bridge NamespaceMode = "bridge"
	// Slirp indicates that a slirp4netns network stack should
	// be used.
	// Only used with the network namespace, invalid otherwise.
	Slirp NamespaceMode = "slirp4netns"
	// Pasta indicates that a pasta network stack should be used.
	// Only used with the network namespace, invalid otherwise.
	Pasta NamespaceMode = "pasta"
	// KeepId indicates a user namespace to keep the owner uid inside
	// of the namespace itself.
	// Only used with the user namespace, invalid otherwise.
	KeepID NamespaceMode = "keep-id"
	// NoMap indicates a user namespace to keep the owner uid out
	// of the namespace itself.
	// Only used with the user namespace, invalid otherwise.
	NoMap NamespaceMode = "no-map"
	// Auto indicates to automatically create a user namespace.
	// Only used with the user namespace, invalid otherwise.
	Auto NamespaceMode = "auto"

	// DefaultKernelNamespaces is a comma-separated list of default kernel
	// namespaces.
	DefaultKernelNamespaces = "ipc,net,uts"
)

// Namespace describes the namespace
type Namespace struct {
	NSMode NamespaceMode `json:"nsmode,omitempty"`
	Value  string        `json:"value,omitempty"`
}

// IsDefault returns whether the namespace is set to the default setting (which
// also includes the empty string).
func (n *Namespace) IsDefault() bool {
	return n.NSMode == Default || n.NSMode == ""
}

// IsHost returns a bool if the namespace is host based
func (n *Namespace) IsHost() bool {
	return n.NSMode == Host
}

// IsNone returns a bool if the namespace is set to none
func (n *Namespace) IsNone() bool {
	return n.NSMode == None
}

// IsBridge returns a bool if the namespace is a Bridge
func (n *Namespace) IsBridge() bool {
	return n.NSMode == Bridge
}

// IsPath indicates via bool if the namespace is based on a path
func (n *Namespace) IsPath() bool {
	return n.NSMode == Path
}

// IsContainer indicates via bool if the namespace is based on a container
func (n *Namespace) IsContainer() bool {
	return n.NSMode == FromContainer
}

// IsPod indicates via bool if the namespace is based on a pod
func (n *Namespace) IsPod() bool {
	return n.NSMode == FromPod
}

// IsPrivate indicates the namespace is private
func (n *Namespace) IsPrivate() bool {
	return n.NSMode == Private
}

// IsAuto indicates the namespace is auto
func (n *Namespace) IsAuto() bool {
	return n.NSMode == Auto
}

// IsKeepID indicates the namespace is KeepID
func (n *Namespace) IsKeepID() bool {
	return n.NSMode == KeepID
}

// IsNoMap indicates the namespace is NoMap
func (n *Namespace) IsNoMap() bool {
	return n.NSMode == NoMap
}

func (n *Namespace) String() string {
	if n.Value != "" {
		return fmt.Sprintf("%s:%s", n.NSMode, n.Value)
	}
	return string(n.NSMode)
}

func validateUserNS(n *Namespace) error {
	if n == nil {
		return nil
	}
	switch n.NSMode {
	case Auto, KeepID, NoMap:
		return nil
	}
	return n.validate()
}

func validateNetNS(n *Namespace) error {
	if n == nil {
		return nil
	}
	switch n.NSMode {
	case Slirp:
		break
	case Pasta:
		if rootless.IsRootless() {
			break
		}
		return fmt.Errorf("pasta networking is only supported for rootless mode")
	case "", Default, Host, Path, FromContainer, FromPod, Private, NoNetwork, Bridge:
		break
	default:
		return fmt.Errorf("invalid network %q", n.NSMode)
	}

	// Path and From Container MUST have a string value set
	if n.NSMode == Path || n.NSMode == FromContainer {
		if len(n.Value) < 1 {
			return fmt.Errorf("namespace mode %s requires a value", n.NSMode)
		}
	} else if n.NSMode != Slirp {
		// All others except must NOT set a string value
		if len(n.Value) > 0 {
			return fmt.Errorf("namespace value %s cannot be provided with namespace mode %s", n.Value, n.NSMode)
		}
	}

	return nil
}

func validateIPCNS(n *Namespace) error {
	if n == nil {
		return nil
	}
	switch n.NSMode {
	case Shareable, None:
		return nil
	}
	return n.validate()
}

// Validate perform simple validation on the namespace to make sure it is not
// invalid from the get-go
func (n *Namespace) validate() error {
	if n == nil {
		return nil
	}
	switch n.NSMode {
	case "", Default, Host, Path, FromContainer, FromPod, Private:
		// Valid, do nothing
	case NoNetwork, Bridge, Slirp, Pasta:
		return errors.New("cannot use network modes with non-network namespace")
	default:
		return fmt.Errorf("invalid namespace type %s specified", n.NSMode)
	}

	// Path and From Container MUST have a string value set
	if n.NSMode == Path || n.NSMode == FromContainer {
		if len(n.Value) < 1 {
			return fmt.Errorf("namespace mode %s requires a value", n.NSMode)
		}
	} else {
		// All others must NOT set a string value
		if len(n.Value) > 0 {
			return fmt.Errorf("namespace value %s cannot be provided with namespace mode %s", n.Value, n.NSMode)
		}
	}
	return nil
}

// ParseNamespace parses a namespace in string form.
// This is not intended for the network namespace, which has a separate
// function.
func ParseNamespace(ns string) (Namespace, error) {
	toReturn := Namespace{}
	switch {
	case ns == "pod":
		toReturn.NSMode = FromPod
	case ns == "host":
		toReturn.NSMode = Host
	case ns == "private", ns == "":
		toReturn.NSMode = Private
	case strings.HasPrefix(ns, "ns:"):
		split := strings.SplitN(ns, ":", 2)
		if len(split) != 2 {
			return toReturn, fmt.Errorf("must provide a path to a namespace when specifying \"ns:\"")
		}
		toReturn.NSMode = Path
		toReturn.Value = split[1]
	case strings.HasPrefix(ns, "container:"):
		split := strings.SplitN(ns, ":", 2)
		if len(split) != 2 {
			return toReturn, fmt.Errorf("must provide name or ID or a container when specifying \"container:\"")
		}
		toReturn.NSMode = FromContainer
		toReturn.Value = split[1]
	default:
		return toReturn, fmt.Errorf("unrecognized namespace mode %s passed", ns)
	}

	return toReturn, nil
}

// ParseCgroupNamespace parses a cgroup namespace specification in string
// form.
func ParseCgroupNamespace(ns string) (Namespace, error) {
	toReturn := Namespace{}
	// Cgroup is host for v1, private for v2.
	// We can't trust c/common for this, as it only assumes private.
	cgroupsv2, err := cgroups.IsCgroup2UnifiedMode()
	if err != nil {
		return toReturn, err
	}
	if cgroupsv2 {
		switch ns {
		case "host":
			toReturn.NSMode = Host
		case "private", "":
			toReturn.NSMode = Private
		default:
			return toReturn, fmt.Errorf("unrecognized cgroup namespace mode %s passed", ns)
		}
	} else {
		toReturn.NSMode = Host
	}
	return toReturn, nil
}

// ParseIPCNamespace parses an ipc namespace specification in string
// form.
func ParseIPCNamespace(ns string) (Namespace, error) {
	toReturn := Namespace{}
	switch {
	case ns == "shareable", ns == "":
		toReturn.NSMode = Shareable
		return toReturn, nil
	case ns == "none":
		toReturn.NSMode = None
		return toReturn, nil
	}
	return ParseNamespace(ns)
}

// ParseUserNamespace parses a user namespace specification in string
// form.
func ParseUserNamespace(ns string) (Namespace, error) {
	toReturn := Namespace{}
	switch {
	case ns == "auto":
		toReturn.NSMode = Auto
		return toReturn, nil
	case strings.HasPrefix(ns, "auto:"):
		split := strings.SplitN(ns, ":", 2)
		if len(split) != 2 {
			return toReturn, errors.New("invalid setting for auto: mode")
		}
		toReturn.NSMode = Auto
		toReturn.Value = split[1]
		return toReturn, nil
	case ns == "keep-id":
		toReturn.NSMode = KeepID
		return toReturn, nil
	case strings.HasPrefix(ns, "keep-id:"):
		split := strings.SplitN(ns, ":", 2)
		if len(split) != 2 {
			return toReturn, errors.New("invalid setting for keep-id: mode")
		}
		toReturn.NSMode = KeepID
		toReturn.Value = split[1]
		return toReturn, nil
	case ns == "nomap":
		toReturn.NSMode = NoMap
		return toReturn, nil
	case ns == "":
		toReturn.NSMode = Host
		return toReturn, nil
	}
	return ParseNamespace(ns)
}

// ParseNetworkFlag parses a network string slice into the network options
// If the input is nil or empty it will use the default setting from containers.conf
// TODO (5.0): Drop pastaNetworkNameExists
func ParseNetworkFlag(networks []string, pastaNetworkNameExists bool) (Namespace, map[string]types.PerNetworkOptions, map[string][]string, error) {
	var networkOptions map[string][]string
	// by default we try to use the containers.conf setting
	// if we get at least one value use this instead
	ns := containerConfig.Containers.NetNS
	if len(networks) > 0 {
		ns = networks[0]
	}

	toReturn := Namespace{}
	podmanNetworks := make(map[string]types.PerNetworkOptions)

	switch {
	case ns == string(Slirp), strings.HasPrefix(ns, string(Slirp)+":"):
		parts := strings.SplitN(ns, ":", 2)
		if len(parts) > 1 {
			networkOptions = make(map[string][]string)
			networkOptions[parts[0]] = strings.Split(parts[1], ",")
		}
		toReturn.NSMode = Slirp
	case ns == string(FromPod):
		toReturn.NSMode = FromPod
	case ns == "" || ns == string(Default) || ns == string(Private):
		toReturn.NSMode = Private
	case ns == string(Bridge), strings.HasPrefix(ns, string(Bridge)+":"):
		toReturn.NSMode = Bridge
		parts := strings.SplitN(ns, ":", 2)
		netOpts := types.PerNetworkOptions{}
		if len(parts) > 1 {
			var err error
			netOpts, err = parseBridgeNetworkOptions(parts[1])
			if err != nil {
				return toReturn, nil, nil, err
			}
		}
		// we have to set the special default network name here
		podmanNetworks["default"] = netOpts

	case ns == string(NoNetwork):
		toReturn.NSMode = NoNetwork
	case ns == string(Host):
		toReturn.NSMode = Host
	case strings.HasPrefix(ns, "ns:"):
		split := strings.SplitN(ns, ":", 2)
		if len(split) != 2 {
			return toReturn, nil, nil, errors.New("must provide a path to a namespace when specifying \"ns:\"")
		}
		toReturn.NSMode = Path
		toReturn.Value = split[1]
	case strings.HasPrefix(ns, string(FromContainer)+":"):
		split := strings.SplitN(ns, ":", 2)
		if len(split) != 2 {
			return toReturn, nil, nil, errors.New("must provide name or ID or a container when specifying \"container:\"")
		}
		toReturn.NSMode = FromContainer
		toReturn.Value = split[1]
	case ns == string(Pasta), strings.HasPrefix(ns, string(Pasta)+":"):
		var parts []string

		if pastaNetworkNameExists {
			goto nextCase
		}

		parts = strings.SplitN(ns, ":", 2)
		if len(parts) > 1 {
			networkOptions = make(map[string][]string)
			networkOptions[parts[0]] = strings.Split(parts[1], ",")
		}
		toReturn.NSMode = Pasta
		break
	nextCase:
		fallthrough
	default:
		// we should have a normal network
		parts := strings.SplitN(ns, ":", 2)
		if len(parts) == 1 {
			// Assume we have been given a comma separated list of networks for backwards compat.
			networkList := strings.Split(ns, ",")
			for _, net := range networkList {
				podmanNetworks[net] = types.PerNetworkOptions{}
			}
		} else {
			if parts[0] == "" {
				return toReturn, nil, nil, errors.New("network name cannot be empty")
			}
			netOpts, err := parseBridgeNetworkOptions(parts[1])
			if err != nil {
				return toReturn, nil, nil, fmt.Errorf("invalid option for network %s: %w", parts[0], err)
			}
			podmanNetworks[parts[0]] = netOpts
		}

		// networks need bridge mode
		toReturn.NSMode = Bridge
	}

	if len(networks) > 1 {
		if !toReturn.IsBridge() {
			return toReturn, nil, nil, fmt.Errorf("cannot set multiple networks without bridge network mode, selected mode %s: %w", toReturn.NSMode, define.ErrInvalidArg)
		}

		for _, network := range networks[1:] {
			parts := strings.SplitN(network, ":", 2)
			if parts[0] == "" {
				return toReturn, nil, nil, fmt.Errorf("network name cannot be empty: %w", define.ErrInvalidArg)
			}
			// TODO (5.0): Don't accept string(Pasta) here once we drop pastaNetworkNameExists
			if cutil.StringInSlice(parts[0], []string{string(Bridge), string(Slirp), string(FromPod), string(NoNetwork),
				string(Default), string(Private), string(Path), string(FromContainer), string(Host)}) {
				return toReturn, nil, nil, fmt.Errorf("can only set extra network names, selected mode %s conflicts with bridge: %w", parts[0], define.ErrInvalidArg)
			}
			netOpts := types.PerNetworkOptions{}
			if len(parts) > 1 {
				var err error
				netOpts, err = parseBridgeNetworkOptions(parts[1])
				if err != nil {
					return toReturn, nil, nil, fmt.Errorf("invalid option for network %s: %w", parts[0], err)
				}
			}
			podmanNetworks[parts[0]] = netOpts
		}
	}

	return toReturn, podmanNetworks, networkOptions, nil
}

func parseBridgeNetworkOptions(opts string) (types.PerNetworkOptions, error) {
	netOpts := types.PerNetworkOptions{}
	if len(opts) == 0 {
		return netOpts, nil
	}
	allopts := strings.Split(opts, ",")
	for _, opt := range allopts {
		split := strings.SplitN(opt, "=", 2)
		switch split[0] {
		case "ip", "ip6":
			ip := net.ParseIP(split[1])
			if ip == nil {
				return netOpts, fmt.Errorf("invalid ip address %q", split[1])
			}
			netOpts.StaticIPs = append(netOpts.StaticIPs, ip)

		case "mac":
			mac, err := net.ParseMAC(split[1])
			if err != nil {
				return netOpts, err
			}
			netOpts.StaticMAC = types.HardwareAddr(mac)

		case "alias":
			if split[1] == "" {
				return netOpts, errors.New("alias cannot be empty")
			}
			netOpts.Aliases = append(netOpts.Aliases, split[1])

		case "interface_name":
			if split[1] == "" {
				return netOpts, errors.New("interface_name cannot be empty")
			}
			netOpts.InterfaceName = split[1]

		default:
			return netOpts, fmt.Errorf("unknown bridge network option: %s", split[0])
		}
	}
	return netOpts, nil
}

func SetupUserNS(idmappings *storage.IDMappingOptions, userns Namespace, g *generate.Generator) (string, error) {
	// User
	var user string
	switch userns.NSMode {
	case Path:
		if _, err := os.Stat(userns.Value); err != nil {
			return user, fmt.Errorf("cannot find specified user namespace path: %w", err)
		}
		if err := g.AddOrReplaceLinuxNamespace(string(spec.UserNamespace), userns.Value); err != nil {
			return user, err
		}
		// runc complains if no mapping is specified, even if we join another ns.  So provide a dummy mapping
		g.AddLinuxUIDMapping(uint32(0), uint32(0), uint32(1))
		g.AddLinuxGIDMapping(uint32(0), uint32(0), uint32(1))
	case Host:
		if err := g.RemoveLinuxNamespace(string(spec.UserNamespace)); err != nil {
			return user, err
		}
	case KeepID:
		opts, err := namespaces.UsernsMode(userns.String()).GetKeepIDOptions()
		if err != nil {
			return user, err
		}
		mappings, uid, gid, err := util.GetKeepIDMapping(opts)
		if err != nil {
			return user, err
		}
		idmappings = mappings
		g.SetProcessUID(uint32(uid))
		g.SetProcessGID(uint32(gid))
		g.AddProcessAdditionalGid(uint32(gid))
		user = fmt.Sprintf("%d:%d", uid, gid)
		if err := privateUserNamespace(idmappings, g); err != nil {
			return user, err
		}
	case NoMap:
		mappings, uid, gid, err := util.GetNoMapMapping()
		if err != nil {
			return user, err
		}
		idmappings = mappings
		g.SetProcessUID(uint32(uid))
		g.SetProcessGID(uint32(gid))
		g.AddProcessAdditionalGid(uint32(gid))
		user = fmt.Sprintf("%d:%d", uid, gid)
		if err := privateUserNamespace(idmappings, g); err != nil {
			return user, err
		}
	case Private:
		if err := privateUserNamespace(idmappings, g); err != nil {
			return user, err
		}
	}
	return user, nil
}

func privateUserNamespace(idmappings *storage.IDMappingOptions, g *generate.Generator) error {
	if err := g.AddOrReplaceLinuxNamespace(string(spec.UserNamespace), ""); err != nil {
		return err
	}
	if idmappings == nil || (len(idmappings.UIDMap) == 0 && len(idmappings.GIDMap) == 0) {
		return errors.New("must provide at least one UID or GID mapping to configure a user namespace")
	}
	for _, uidmap := range idmappings.UIDMap {
		g.AddLinuxUIDMapping(uint32(uidmap.HostID), uint32(uidmap.ContainerID), uint32(uidmap.Size))
	}
	for _, gidmap := range idmappings.GIDMap {
		g.AddLinuxGIDMapping(uint32(gidmap.HostID), uint32(gidmap.ContainerID), uint32(gidmap.Size))
	}
	return nil
}
