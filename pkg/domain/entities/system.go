package entities

import (
	"time"

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
	All    bool
	Volume bool
}

// SystemPruneReport provides report after system prune is executed.
type SystemPruneReport struct {
	PodPruneReport []*PodPruneReport
	*ContainerPruneReport
	*ImagePruneReport
	VolumePruneReport []*VolumePruneReport
}
