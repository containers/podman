//go:build freebsd
// +build freebsd

package jail

import (
	"strings"
	"syscall"
	"unsafe"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

type NS int32

const (
	DISABLED NS = 0
	NEW      NS = 1
	INHERIT  NS = 2

	JAIL_CREATE = 0x01
	JAIL_UPDATE = 0x02
	JAIL_ATTACH = 0x04
)

type config struct {
	params map[string]interface{}
}

func NewConfig() *config {
	return &config{
		params: make(map[string]interface{}),
	}
}

func handleBoolSetting(key string, val bool) (string, interface{}) {
	// jail doesn't deal with booleans - it uses paired parameter
	// names, e.g. "persist"/"nopersist". If the key contains '.',
	// the "no" prefix is applied to the last element.
	if val == false {
		parts := strings.Split(key, ".")
		parts[len(parts)-1] = "no" + parts[len(parts)-1]
		key = strings.Join(parts, ".")
	}
	return key, nil
}

func (c *config) Set(key string, value interface{}) {
	// Normalise integer types to int32
	switch v := value.(type) {
	case int:
		value = int32(v)
	case uint32:
		value = int32(v)
	}

	switch key {
	case "jid", "devfs_ruleset", "enforce_statfs", "children.max", "securelevel":
		if _, ok := value.(int32); !ok {
			logrus.Fatalf("value for parameter %s must be an int32", key)
		}
	case "ip4", "ip6", "host", "vnet":
		nsval, ok := value.(NS)
		if !ok {
			logrus.Fatalf("value for parameter %s must be a jail.NS", key)
		}
		if (key == "host" || key == "vnet") && nsval == DISABLED {
			logrus.Fatalf("value for parameter %s cannot be DISABLED", key)
		}
	case "persist", "sysvmsg", "sysvsem", "sysvshm":
		bval, ok := value.(bool)
		if !ok {
			logrus.Fatalf("value for parameter %s must be bool", key)
		}
		key, value = handleBoolSetting(key, bval)
	default:
		if strings.HasPrefix(key, "allow.") {
			bval, ok := value.(bool)
			if !ok {
				logrus.Fatalf("value for parameter %s must be bool", key)
			}
			key, value = handleBoolSetting(key, bval)
		} else {
			if _, ok := value.(string); !ok {
				logrus.Fatalf("value for parameter %s must be a string", key)
			}
		}
	}
	c.params[key] = value
}

func (c *config) getIovec() ([]syscall.Iovec, error) {
	jiov := make([]syscall.Iovec, 0)
	for key, value := range c.params {
		iov, err := stringToIovec(key)
		if err != nil {
			return nil, err
		}
		jiov = append(jiov, iov)
		switch v := value.(type) {
		case string:
			iov, err := stringToIovec(v)
			if err != nil {
				return nil, err
			}
			jiov = append(jiov, iov)
		case int32:
			jiov = append(jiov, syscall.Iovec{
				Base: (*byte)(unsafe.Pointer(&v)),
				Len:  4,
			})
		case NS:
			jiov = append(jiov, syscall.Iovec{
				Base: (*byte)(unsafe.Pointer(&v)),
				Len:  4,
			})
		default:
			jiov = append(jiov, syscall.Iovec{
				Base: nil,
				Len:  0,
			})
		}
	}
	return jiov, nil
}

type jail struct {
	jid int32
}

func jailSet(jconf *config, flags int) (*jail, error) {
	jiov, err := jconf.getIovec()
	if err != nil {
		return nil, err
	}

	jid, _, errno := syscall.Syscall(unix.SYS_JAIL_SET, uintptr(unsafe.Pointer(&jiov[0])), uintptr(len(jiov)), uintptr(flags))
	if errno != 0 {
		return nil, errno
	}
	return &jail{
		jid: int32(jid),
	}, nil
}

func jailGet(jconf *config, flags int) (*jail, error) {
	jiov, err := jconf.getIovec()
	if err != nil {
		return nil, err
	}

	jid, _, errno := syscall.Syscall(unix.SYS_JAIL_GET, uintptr(unsafe.Pointer(&jiov[0])), uintptr(len(jiov)), uintptr(flags))
	if errno != 0 {
		return nil, errno
	}
	return &jail{
		jid: int32(jid),
	}, nil
}

func Create(jconf *config) (*jail, error) {
	return jailSet(jconf, JAIL_CREATE)
}

func CreateAndAttach(jconf *config) (*jail, error) {
	return jailSet(jconf, JAIL_CREATE|JAIL_ATTACH)
}

func FindByName(name string) (*jail, error) {
	jconf := NewConfig()
	jconf.Set("name", name)
	return jailGet(jconf, 0)
}

func (j *jail) Set(jconf *config) error {
	jconf.Set("jid", j.jid)
	_, err := jailSet(jconf, JAIL_UPDATE)
	return err
}
