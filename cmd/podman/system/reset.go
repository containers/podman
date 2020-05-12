package system

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/cmd/podman/validate"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/domain/infra"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	systemResetDescription = `Reset podman storage back to default state"

  All containers will be stopped and removed, and all images, volumes and container content will be removed.
`
	systemResetCommand = &cobra.Command{
		Use:   "reset",
		Args:  validate.NoArgs,
		Short: "Reset podman storage",
		Long:  systemResetDescription,
		Run:   reset,
	}

	forceFlag bool
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: systemResetCommand,
		Parent:  systemCmd,
	})
	flags := systemResetCommand.Flags()
	flags.BoolVarP(&forceFlag, "force", "f", false, "Do not prompt for confirmation")
}

func reset(cmd *cobra.Command, args []string) {
	// Prompt for confirmation if --force is not set
	if !forceFlag {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print(`
WARNING! This will remove:
        - all containers
        - all pods
        - all images
        - all build cache
Are you sure you want to continue? [y/N] `)
		answer, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println(errors.Wrapf(err, "error reading input"))
			os.Exit(1)
		}
		if strings.ToLower(answer)[0] != 'y' {
			os.Exit(0)
		}
	}

	// Shutdown all running engines, `reset` will hijack repository
	registry.ContainerEngine().Shutdown(registry.Context())
	registry.ImageEngine().Shutdown(registry.Context())

	engine, err := infra.NewSystemEngine(entities.ResetMode, registry.PodmanConfig())
	if err != nil {
		fmt.Println(err)
		os.Exit(125)
	}
	defer engine.Shutdown(registry.Context())

	if err := engine.Reset(registry.Context()); err != nil {
		fmt.Println(err)
		os.Exit(125)
	}
	os.Exit(0)
}
