package namespaces

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/containers/storage/types"
)

const (
	bridgeType    = "bridge"
	containerType = "container"
	defaultType   = "default"
	hostType      = "host"
	noneType      = "none"
	nsType        = "ns"
	podType       = "pod"
	privateType   = "private"
	shareableType = "shareable"
	slirpType     = "slirp4netns"
)

// CgroupMode represents cgroup mode in the container.
type CgroupMode string

// IsHost indicates whether the container uses the host's cgroup.
func (n CgroupMode) IsHost() bool {
	return n == hostType
}

// IsDefaultValue indicates whether the cgroup namespace has the default value.
func (n CgroupMode) IsDefaultValue() bool {
	return n == "" || n == defaultType
}

// IsNS indicates a cgroup namespace passed in by path (ns:<path>)
func (n CgroupMode) IsNS() bool {
	return strings.HasPrefix(string(n), nsType)
}

// NS gets the path associated with a ns:<path> cgroup ns
func (n CgroupMode) NS() string {
	parts := strings.SplitN(string(n), ":", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// IsContainer indicates whether the container uses a new cgroup namespace.
func (n CgroupMode) IsContainer() bool {
	parts := strings.SplitN(string(n), ":", 2)
	return len(parts) > 1 && parts[0] == containerType
}

// Container returns the name of the container whose cgroup namespace is going to be used.
func (n CgroupMode) Container() string {
	parts := strings.SplitN(string(n), ":", 2)
	if len(parts) > 1 && parts[0] == containerType {
		return parts[1]
	}
	return ""
}

// IsPrivate indicates whether the container uses the a private cgroup.
func (n CgroupMode) IsPrivate() bool {
	return n == privateType
}

// Valid indicates whether the Cgroup namespace is valid.
func (n CgroupMode) Valid() bool {
	parts := strings.Split(string(n), ":")
	switch mode := parts[0]; mode {
	case "", hostType, privateType, nsType:
	case containerType:
		if len(parts) != 2 || parts[1] == "" {
			return false
		}
	default:
		return false
	}
	return true
}

// UsernsMode represents userns mode in the container.
type UsernsMode string

// IsHost indicates whether the container uses the host's userns.
func (n UsernsMode) IsHost() bool {
	return n == hostType
}

// IsKeepID indicates whether container uses a mapping where the (uid, gid) on the host is kept inside of the namespace.
func (n UsernsMode) IsKeepID() bool {
	return n == "keep-id"
}

// IsAuto indicates whether container uses the "auto" userns mode.
func (n UsernsMode) IsAuto() bool {
	parts := strings.Split(string(n), ":")
	return parts[0] == "auto"
}

// IsDefaultValue indicates whether the user namespace has the default value.
func (n UsernsMode) IsDefaultValue() bool {
	return n == "" || n == defaultType
}

// GetAutoOptions returns a AutoUserNsOptions with the settings to setup automatically
// a user namespace.
func (n UsernsMode) GetAutoOptions() (*types.AutoUserNsOptions, error) {
	parts := strings.SplitN(string(n), ":", 2)
	if parts[0] != "auto" {
		return nil, fmt.Errorf("wrong user namespace mode")
	}
	options := types.AutoUserNsOptions{}
	if len(parts) == 1 {
		return &options, nil
	}
	for _, o := range strings.Split(parts[1], ",") {
		v := strings.SplitN(o, "=", 2)
		if len(v) != 2 {
			return nil, fmt.Errorf("invalid option specified: %q", o)
		}
		switch v[0] {
		case "size":
			s, err := strconv.ParseUint(v[1], 10, 32)
			if err != nil {
				return nil, err
			}
			options.Size = uint32(s)
		case "uidmapping":
			mapping, err := types.ParseIDMapping([]string{v[1]}, nil, "", "")
			if err != nil {
				return nil, err
			}
			options.AdditionalUIDMappings = append(options.AdditionalUIDMappings, mapping.UIDMap...)
		case "gidmapping":
			mapping, err := types.ParseIDMapping(nil, []string{v[1]}, "", "")
			if err != nil {
				return nil, err
			}
			options.AdditionalGIDMappings = append(options.AdditionalGIDMappings, mapping.GIDMap...)
		default:
			return nil, fmt.Errorf("unknown option specified: %q", v[0])
		}
	}
	return &options, nil
}

// IsPrivate indicates whether the container uses the a private userns.
func (n UsernsMode) IsPrivate() bool {
	return !(n.IsHost() || n.IsContainer())
}

// Valid indicates whether the userns is valid.
func (n UsernsMode) Valid() bool {
	parts := strings.Split(string(n), ":")
	switch mode := parts[0]; mode {
	case "", privateType, hostType, "keep-id", nsType, "auto":
	case containerType:
		if len(parts) != 2 || parts[1] == "" {
			return false
		}
	default:
		return false
	}
	return true
}

// IsNS indicates a userns namespace passed in by path (ns:<path>)
func (n UsernsMode) IsNS() bool {
	return strings.HasPrefix(string(n), "ns:")
}

// NS gets the path associated with a ns:<path> userns ns
func (n UsernsMode) NS() string {
	parts := strings.SplitN(string(n), ":", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// IsContainer indicates whether container uses a container userns.
func (n UsernsMode) IsContainer() bool {
	parts := strings.SplitN(string(n), ":", 2)
	return len(parts) > 1 && parts[0] == containerType
}

// Container is the id of the container which network this container is connected to.
func (n UsernsMode) Container() string {
	parts := strings.SplitN(string(n), ":", 2)
	if len(parts) > 1 && parts[0] == containerType {
		return parts[1]
	}
	return ""
}

// UTSMode represents the UTS namespace of the container.
type UTSMode string

// IsPrivate indicates whether the container uses its private UTS namespace.
func (n UTSMode) IsPrivate() bool {
	return !(n.IsHost())
}

// IsHost indicates whether the container uses the host's UTS namespace.
func (n UTSMode) IsHost() bool {
	return n == hostType
}

// IsContainer indicates whether the container uses a container's UTS namespace.
func (n UTSMode) IsContainer() bool {
	parts := strings.SplitN(string(n), ":", 2)
	return len(parts) > 1 && parts[0] == containerType
}

// Container returns the name of the container whose uts namespace is going to be used.
func (n UTSMode) Container() string {
	parts := strings.SplitN(string(n), ":", 2)
	if len(parts) > 1 && parts[0] == containerType {
		return parts[1]
	}
	return ""
}

// Valid indicates whether the UTS namespace is valid.
func (n UTSMode) Valid() bool {
	parts := strings.Split(string(n), ":")
	switch mode := parts[0]; mode {
	case "", privateType, hostType:
	case containerType:
		if len(parts) != 2 || parts[1] == "" {
			return false
		}
	default:
		return false
	}
	return true
}

// IpcMode represents the container ipc stack.
type IpcMode string

// IsPrivate indicates whether the container uses its own private ipc namespace which cannot be shared.
func (n IpcMode) IsPrivate() bool {
	return n == privateType
}

// IsHost indicates whether the container shares the host's ipc namespace.
func (n IpcMode) IsHost() bool {
	return n == hostType
}

// IsShareable indicates whether the container's ipc namespace can be shared with another container.
func (n IpcMode) IsShareable() bool {
	return n == shareableType
}

// IsContainer indicates whether the container uses another container's ipc namespace.
func (n IpcMode) IsContainer() bool {
	parts := strings.SplitN(string(n), ":", 2)
	return len(parts) > 1 && parts[0] == containerType
}

// IsNone indicates whether container IpcMode is set to "none".
func (n IpcMode) IsNone() bool {
	return n == noneType
}

// IsEmpty indicates whether container IpcMode is empty
func (n IpcMode) IsEmpty() bool {
	return n == ""
}

// Valid indicates whether the ipc mode is valid.
func (n IpcMode) Valid() bool {
	return n.IsEmpty() || n.IsNone() || n.IsPrivate() || n.IsHost() || n.IsShareable() || n.IsContainer()
}

// Container returns the name of the container ipc stack is going to be used.
func (n IpcMode) Container() string {
	parts := strings.SplitN(string(n), ":", 2)
	if len(parts) > 1 && parts[0] == containerType {
		return parts[1]
	}
	return ""
}

// PidMode represents the pid namespace of the container.
type PidMode string

// IsPrivate indicates whether the container uses its own new pid namespace.
func (n PidMode) IsPrivate() bool {
	return !(n.IsHost() || n.IsContainer())
}

// IsHost indicates whether the container uses the host's pid namespace.
func (n PidMode) IsHost() bool {
	return n == hostType
}

// IsContainer indicates whether the container uses a container's pid namespace.
func (n PidMode) IsContainer() bool {
	parts := strings.SplitN(string(n), ":", 2)
	return len(parts) > 1 && parts[0] == containerType
}

// Valid indicates whether the pid namespace is valid.
func (n PidMode) Valid() bool {
	parts := strings.Split(string(n), ":")
	switch mode := parts[0]; mode {
	case "", privateType, hostType:
	case containerType:
		if len(parts) != 2 || parts[1] == "" {
			return false
		}
	default:
		return false
	}
	return true
}

// Container returns the name of the container whose pid namespace is going to be used.
func (n PidMode) Container() string {
	parts := strings.SplitN(string(n), ":", 2)
	if len(parts) > 1 && parts[0] == containerType {
		return parts[1]
	}
	return ""
}

// NetworkMode represents the container network stack.
type NetworkMode string

// IsNone indicates whether container isn't using a network stack.
func (n NetworkMode) IsNone() bool {
	return n == noneType
}

// IsHost indicates whether the container uses the host's network stack.
func (n NetworkMode) IsHost() bool {
	return n == hostType
}

// IsDefault indicates whether container uses the default network stack.
func (n NetworkMode) IsDefault() bool {
	return n == defaultType
}

// IsPrivate indicates whether container uses its private network stack.
func (n NetworkMode) IsPrivate() bool {
	return !(n.IsHost() || n.IsContainer())
}

// IsContainer indicates whether container uses a container network stack.
func (n NetworkMode) IsContainer() bool {
	parts := strings.SplitN(string(n), ":", 2)
	return len(parts) > 1 && parts[0] == containerType
}

// Container is the id of the container which network this container is connected to.
func (n NetworkMode) Container() string {
	parts := strings.SplitN(string(n), ":", 2)
	if len(parts) > 1 && parts[0] == containerType {
		return parts[1]
	}
	return ""
}

//UserDefined indicates user-created network
func (n NetworkMode) UserDefined() string {
	if n.IsUserDefined() {
		return string(n)
	}
	return ""
}

// IsBridge indicates whether container uses the bridge network stack
func (n NetworkMode) IsBridge() bool {
	return n == bridgeType
}

// IsSlirp4netns indicates if we are running a rootless network stack
func (n NetworkMode) IsSlirp4netns() bool {
	return n == slirpType || strings.HasPrefix(string(n), slirpType+":")
}

// IsNS indicates a network namespace passed in by path (ns:<path>)
func (n NetworkMode) IsNS() bool {
	return strings.HasPrefix(string(n), nsType)
}

// NS gets the path associated with a ns:<path> network ns
func (n NetworkMode) NS() string {
	parts := strings.SplitN(string(n), ":", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// IsPod returns whether the network refers to pod networking
func (n NetworkMode) IsPod() bool {
	return n == podType
}

// IsUserDefined indicates user-created network
func (n NetworkMode) IsUserDefined() bool {
	return !n.IsDefault() && !n.IsBridge() && !n.IsHost() && !n.IsNone() && !n.IsContainer() && !n.IsSlirp4netns() && !n.IsNS()
}
