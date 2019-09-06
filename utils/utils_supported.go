// +build linux darwin

package utils

import (
	"github.com/containers/libpod/pkg/cgroups"
	"github.com/containers/libpod/pkg/rootless"
	systemdDbus "github.com/coreos/go-systemd/dbus"
	"github.com/godbus/dbus"
)

// RunUnderSystemdScope adds the specified pid to a systemd scope
func RunUnderSystemdScope(pid int, slice string, unitName string) error {
	var properties []systemdDbus.Property
	var conn *systemdDbus.Conn
	var err error

	if rootless.IsRootless() {
		conn, err = cgroups.GetUserConnection(rootless.GetRootlessUID())
		if err != nil {
			return err
		}
	} else {
		conn, err = systemdDbus.New()
		if err != nil {
			return err
		}
	}
	properties = append(properties, systemdDbus.PropSlice(slice))
	properties = append(properties, newProp("PIDs", []uint32{uint32(pid)}))
	properties = append(properties, newProp("Delegate", true))
	properties = append(properties, newProp("DefaultDependencies", false))
	ch := make(chan string)
	_, err = conn.StartTransientUnit(unitName, "replace", properties, ch)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Block until job is started
	<-ch

	return nil
}

func newProp(name string, units interface{}) systemdDbus.Property {
	return systemdDbus.Property{
		Name:  name,
		Value: dbus.MakeVariant(units),
	}
}
