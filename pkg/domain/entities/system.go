package entities

import (
	"time"

	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/pkg/domain/entities/reports"
	"github.com/containers/podman/v3/pkg/domain/entities/types"
	"github.com/spf13/cobra"
)

// ServiceOptions provides the input for starting an API Service
type ServiceOptions struct {
	URI     string         // Path to unix domain socket service should listen on
	Timeout time.Duration  // duration of inactivity the service should wait before shutting down
	Command *cobra.Command // CLI command provided. Used in V1 code
}

// SystemPruneOptions provides options to prune system.
type SystemPruneOptions struct {
	All     bool
	Volume  bool
	Filters map[string][]string `json:"filters" schema:"filters"`
}

// SystemPruneReport provides report after system prune is executed.
type SystemPruneReport struct {
	PodPruneReport        []*PodPruneReport
	ContainerPruneReports []*reports.PruneReport
	ImagePruneReports     []*reports.PruneReport
	VolumePruneReports    []*reports.PruneReport
	ReclaimedSpace        uint64
}

// SystemMigrateOptions describes the options needed for the
// cli to migrate runtimes of containers
type SystemMigrateOptions struct {
	NewRuntime string
}

// SystemDfOptions describes the options for getting df information
type SystemDfOptions struct {
	Format  string
	Verbose bool
}

// SystemDfReport describes the response for df information
type SystemDfReport struct {
	Images     []*SystemDfImageReport
	Containers []*SystemDfContainerReport
	Volumes    []*SystemDfVolumeReport
}

// SystemDfImageReport describes an image for use with df
type SystemDfImageReport struct {
	Repository string
	Tag        string
	ImageID    string
	Created    time.Time
	Size       int64
	SharedSize int64
	UniqueSize int64
	Containers int
}

// SystemDfContainerReport describes a container for use with df
type SystemDfContainerReport struct {
	ContainerID  string
	Image        string
	Command      []string
	LocalVolumes int
	Size         int64
	RWSize       int64
	Created      time.Time
	Status       string
	Names        string
}

// SystemDfVolumeReport describes a volume and its size
type SystemDfVolumeReport struct {
	VolumeName      string
	Links           int
	Size            int64
	ReclaimableSize int64
}

// SystemResetOptions describes the options for resetting your
// container runtime storage, etc
type SystemResetOptions struct {
	Force bool
}

// SystemVersionReport describes version information about the running Podman service
type SystemVersionReport struct {
	// Always populated
	Client *define.Version `json:",omitempty"`
	// May be populated, when in tunnel mode
	Server *define.Version `json:",omitempty"`
}

// SystemUnshareOptions describes the options for the unshare command
type SystemUnshareOptions struct {
	RootlessCNI bool
}

type ComponentVersion struct {
	types.Version
}

// ListRegistriesReport is the report when querying for a sorted list of
// registries which may be contacted during certain operations.
type ListRegistriesReport struct {
	Registries []string
}

// swagger:model AuthConfig
type AuthConfig struct {
	types.AuthConfig
}

// AuthReport describes the response for authentication check
type AuthReport struct {
	IdentityToken string
	Status        string
}
