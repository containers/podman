package farm

import (
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/spf13/cobra"
)

var (
	// Command: podman _farm_
	farmCmd = &cobra.Command{
		Use:   "farm",
		Short: "Farm out builds to remote machines",
		Long:  "Farm out builds to remote machines that podman can connect to via podman system connection",
		RunE:  validate.SubCommandExists,
	}
)

var (
	// Temporary struct to hold cli values.
	farmOpts = struct {
		Farm  string
		Local bool
	}{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: farmCmd,
	})
	farmCmd.Hidden = true

	flags := farmCmd.Flags()
	podmanConfig := registry.PodmanConfig()

	farmFlagName := "farm"
	// If remote, don't read the client's containers.conf file
	defaultFarm := ""
	if !registry.IsRemote() {
		defaultFarm = podmanConfig.ContainersConfDefaultsRO.Farms.Default
	}
	flags.StringVarP(&farmOpts.Farm, farmFlagName, "f", defaultFarm, "Farm to use for builds")

	localFlagName := "local"
	// Default for local is true and hide this flag for the remote use case
	if !registry.IsRemote() {
		flags.BoolVarP(&farmOpts.Local, localFlagName, "l", true, "Build image on local machine including on farm nodes")
	}
}
