package specgen

import (
	"strings"

	"github.com/containers/podman/v3/pkg/cgroups"
	"github.com/containers/podman/v3/pkg/rootless"
	"github.com/pkg/errors"
)

type NamespaceMode string

const (
	// Default indicates the spec generator should determine
	// a sane default
	Default NamespaceMode = "default"
	// Host means the the namespace is derived from
	// the host
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
	// NoNetwork indicates no network namespace should
	// be joined.  loopback should still exists.
	// Only used with the network namespace, invalid otherwise.
	NoNetwork NamespaceMode = "none"
	// Bridge indicates that a CNI network stack
	// should be used.
	// Only used with the network namespace, invalid otherwise.
	Bridge NamespaceMode = "bridge"
	// Slirp indicates that a slirp4netns network stack should
	// be used.
	// Only used with the network namespace, invalid otherwise.
	Slirp NamespaceMode = "slirp4netns"
	// KeepId indicates a user namespace to keep the owner uid inside
	// of the namespace itself.
	// Only used with the user namespace, invalid otherwise.
	KeepID NamespaceMode = "keep-id"
	// Auto indicates to automatically create a user namespace.
	// Only used with the user namespace, invalid otherwise.
	Auto NamespaceMode = "auto"

	// DefaultKernelNamespaces is a comma-separated list of default kernel
	// namespaces.
	DefaultKernelNamespaces = "cgroup,ipc,net,uts"
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

func validateUserNS(n *Namespace) error {
	if n == nil {
		return nil
	}
	switch n.NSMode {
	case Auto, KeepID:
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
	case "", Default, Host, Path, FromContainer, FromPod, Private, NoNetwork, Bridge:
		break
	default:
		return errors.Errorf("invalid network %q", n.NSMode)
	}

	// Path and From Container MUST have a string value set
	if n.NSMode == Path || n.NSMode == FromContainer {
		if len(n.Value) < 1 {
			return errors.Errorf("namespace mode %s requires a value", n.NSMode)
		}
	} else if n.NSMode != Slirp {
		// All others except must NOT set a string value
		if len(n.Value) > 0 {
			return errors.Errorf("namespace value %s cannot be provided with namespace mode %s", n.Value, n.NSMode)
		}
	}

	return nil
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
	case NoNetwork, Bridge, Slirp:
		return errors.Errorf("cannot use network modes with non-network namespace")
	default:
		return errors.Errorf("invalid namespace type %s specified", n.NSMode)
	}

	// Path and From Container MUST have a string value set
	if n.NSMode == Path || n.NSMode == FromContainer {
		if len(n.Value) < 1 {
			return errors.Errorf("namespace mode %s requires a value", n.NSMode)
		}
	} else {
		// All others must NOT set a string value
		if len(n.Value) > 0 {
			return errors.Errorf("namespace value %s cannot be provided with namespace mode %s", n.Value, n.NSMode)
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
			return toReturn, errors.Errorf("must provide a path to a namespace when specifying ns:")
		}
		toReturn.NSMode = Path
		toReturn.Value = split[1]
	case strings.HasPrefix(ns, "container:"):
		split := strings.SplitN(ns, ":", 2)
		if len(split) != 2 {
			return toReturn, errors.Errorf("must provide name or ID or a container when specifying container:")
		}
		toReturn.NSMode = FromContainer
		toReturn.Value = split[1]
	default:
		return toReturn, errors.Errorf("unrecognized namespace mode %s passed", ns)
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
			return toReturn, errors.Errorf("unrecognized namespace mode %s passed", ns)
		}
	} else {
		toReturn.NSMode = Host
	}
	return toReturn, nil
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
			return toReturn, errors.Errorf("invalid setting for auto: mode")
		}
		toReturn.NSMode = Auto
		toReturn.Value = split[1]
		return toReturn, nil
	case ns == "keep-id":
		toReturn.NSMode = KeepID
		return toReturn, nil
	case ns == "":
		toReturn.NSMode = Host
		return toReturn, nil
	}
	return ParseNamespace(ns)
}

// ParseNetworkNamespace parses a network namespace specification in string
// form.
// Returns a namespace and (optionally) a list of CNI networks to join.
func ParseNetworkNamespace(ns string) (Namespace, []string, error) {
	toReturn := Namespace{}
	var cniNetworks []string
	// Net defaults to Slirp on rootless
	switch {
	case ns == string(Slirp), strings.HasPrefix(ns, string(Slirp)+":"):
		toReturn.NSMode = Slirp
	case ns == string(FromPod):
		toReturn.NSMode = FromPod
	case ns == "" || ns == string(Default) || ns == string(Private):
		if rootless.IsRootless() {
			toReturn.NSMode = Slirp
		} else {
			toReturn.NSMode = Bridge
		}
	case ns == string(Bridge):
		toReturn.NSMode = Bridge
	case ns == string(NoNetwork):
		toReturn.NSMode = NoNetwork
	case ns == string(Host):
		toReturn.NSMode = Host
	case strings.HasPrefix(ns, "ns:"):
		split := strings.SplitN(ns, ":", 2)
		if len(split) != 2 {
			return toReturn, nil, errors.Errorf("must provide a path to a namespace when specifying ns:")
		}
		toReturn.NSMode = Path
		toReturn.Value = split[1]
	case strings.HasPrefix(ns, string(FromContainer)+":"):
		split := strings.SplitN(ns, ":", 2)
		if len(split) != 2 {
			return toReturn, nil, errors.Errorf("must provide name or ID or a container when specifying container:")
		}
		toReturn.NSMode = FromContainer
		toReturn.Value = split[1]
	default:
		// Assume we have been given a list of CNI networks.
		// Which only works in bridge mode, so set that.
		cniNetworks = strings.Split(ns, ",")
		toReturn.NSMode = Bridge
	}

	return toReturn, cniNetworks, nil
}
