// +build linux darwin

package utils

import (
	"syscall"

	systemdDbus "github.com/coreos/go-systemd/dbus"
	"github.com/godbus/dbus"
)

// RunUnderSystemdScope adds the specified pid to a systemd scope.
// If forConmon is set, timeout is increased, and stop signal is set to SIGUSR1.
func RunUnderSystemdScope(pid int, slice string, unitName string, forConmon bool) error {
	var properties []systemdDbus.Property
	conn, err := systemdDbus.New()
	if err != nil {
		return err
	}
	properties = append(properties, systemdDbus.PropSlice(slice))
	properties = append(properties, newProp("PIDs", []uint32{uint32(pid)}))
	properties = append(properties, newProp("Delegate", true))
	properties = append(properties, newProp("DefaultDependencies", false))
	if forConmon {
		// 10 minute stop timeout
		var timeout uint64 = 1000000 * 60 * 10
		properties = append(properties, newProp("TimeoutStopUSec", &timeout))
		properties = append(properties, newProp("KillSignal", syscall.SIGUSR1))
	}
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
