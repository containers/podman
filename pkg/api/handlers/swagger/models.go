//nolint:deadcode,unused // these types are used to wire generated swagger to API code
package swagger

import (
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/docker/docker/api/types"
)

// Details for creating a volume
// swagger:model
type volumeCreate struct {
	// Name of the volume driver to use.
	// Required: true
	Driver string `json:"Driver"`

	// A mapping of driver options and values. These options are
	// passed directly to the driver and are driver specific.
	//
	// Required: true
	DriverOpts map[string]string `json:"DriverOpts"`

	// User-defined key/value metadata.
	// Required: true
	Labels map[string]string `json:"Labels"`

	// The new volume's name. If not specified, Docker generates a name.
	//
	// Required: true
	Name string `json:"Name"`
}

// Network create
// swagger:model
type networkCreate types.NetworkCreateRequest

// Network connect
// swagger:model
type networkConnectRequest types.NetworkConnect

// Network disconnect
// swagger:model
type networkDisconnectRequest types.NetworkDisconnect

// Network connect
// swagger:model
type networkConnectRequestLibpod entities.NetworkConnectOptions
