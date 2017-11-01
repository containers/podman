package cgroups

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	systemdDbus "github.com/coreos/go-systemd/dbus"
	"github.com/godbus/dbus"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const (
	SystemdDbus  Name = "systemd"
	defaultSlice      = "system.slice"
)

func Systemd() ([]Subsystem, error) {
	root, err := v1MountPoint()
	if err != nil {
		return nil, err
	}
	defaultSubsystems, err := defaults(root)
	if err != nil {
		return nil, err
	}
	s, err := NewSystemd(root)
	if err != nil {
		return nil, err
	}
	// make sure the systemd controller is added first
	return append([]Subsystem{s}, defaultSubsystems...), nil
}

func Slice(slice, name string) Path {
	if slice == "" {
		slice = defaultSlice
	}
	return func(subsystem Name) (string, error) {
		return filepath.Join(slice, unitName(name)), nil
	}
}

func NewSystemd(root string) (*SystemdController, error) {
	conn, err := systemdDbus.New()
	if err != nil {
		return nil, err
	}
	return &SystemdController{
		root: root,
		conn: conn,
	}, nil
}

type SystemdController struct {
	mu   sync.Mutex
	conn *systemdDbus.Conn
	root string
}

func (s *SystemdController) Name() Name {
	return SystemdDbus
}

func (s *SystemdController) Create(path string, resources *specs.LinuxResources) error {
	slice, name := splitName(path)
	properties := []systemdDbus.Property{
		systemdDbus.PropDescription(fmt.Sprintf("cgroup %s", name)),
		systemdDbus.PropWants(slice),
		newProperty("DefaultDependencies", false),
		newProperty("Delegate", true),
		newProperty("MemoryAccounting", true),
		newProperty("CPUAccounting", true),
		newProperty("BlockIOAccounting", true),
	}
	_, err := s.conn.StartTransientUnit(name, "replace", properties, nil)
	return err
}

func (s *SystemdController) Delete(path string) error {
	_, name := splitName(path)
	_, err := s.conn.StopUnit(name, "replace", nil)
	return err
}

func newProperty(name string, units interface{}) systemdDbus.Property {
	return systemdDbus.Property{
		Name:  name,
		Value: dbus.MakeVariant(units),
	}
}

func unitName(name string) string {
	return fmt.Sprintf("%s.slice", name)
}

func splitName(path string) (slice string, unit string) {
	slice, unit = filepath.Split(path)
	return strings.TrimSuffix(slice, "/"), unit
}
