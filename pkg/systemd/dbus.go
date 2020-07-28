package systemd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/containers/podman/v2/pkg/rootless"
	"github.com/coreos/go-systemd/v22/dbus"
	godbus "github.com/godbus/dbus/v5"
)

func dbusAuthRootlessConnection(createBus func(opts ...godbus.ConnOption) (*godbus.Conn, error)) (*godbus.Conn, error) {
	conn, err := createBus()
	if err != nil {
		return nil, err
	}

	methods := []godbus.Auth{godbus.AuthExternal(strconv.Itoa(rootless.GetRootlessUID()))}

	err = conn.Auth(methods)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

func newRootlessConnection() (*dbus.Conn, error) {
	return dbus.NewConnection(func() (*godbus.Conn, error) {
		return dbusAuthRootlessConnection(func(opts ...godbus.ConnOption) (*godbus.Conn, error) {
			path := filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "systemd/private")
			return godbus.Dial(fmt.Sprintf("unix:path=%s", path))
		})
	})
}

// ConnectToDBUS returns a DBUS connection.  It works both as root and non-root
// users.
func ConnectToDBUS() (*dbus.Conn, error) {
	if rootless.IsRootless() {
		return newRootlessConnection()
	}
	return dbus.NewSystemdConnection()
}
