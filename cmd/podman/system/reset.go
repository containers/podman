// +build !remote

package system

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/infra"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	systemResetDescription = `Reset podman storage back to default state"

  All containers will be stopped and removed, and all images, volumes, networks and container content will be removed.
`
	systemResetCommand = &cobra.Command{
		Annotations:       map[string]string{registry.EngineMode: registry.ABIMode},
		Use:               "reset [options]",
		Args:              validate.NoArgs,
		Short:             "Reset podman storage",
		Long:              systemResetDescription,
		Run:               reset,
		ValidArgsFunction: completion.AutocompleteNone,
	}

	forceFlag bool
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: systemResetCommand,
		Parent:  systemCmd,
	})
	flags := systemResetCommand.Flags()
	flags.BoolVarP(&forceFlag, "force", "f", false, "Do not prompt for confirmation")
}

func reset(cmd *cobra.Command, args []string) {
	// Get all the external containers in use
	listCtn, _ := registry.ContainerEngine().ContainerListExternal(registry.Context())
	listCtnIds := make([]string, 0, len(listCtn))
	for _, externalCtn := range listCtn {
		listCtnIds = append(listCtnIds, externalCtn.ID)
	}
	// Prompt for confirmation if --force is not set
	if !forceFlag {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println(`WARNING! This will remove:
        - all containers
        - all pods
        - all images
        - all networks
        - all build cache`)
		if len(listCtn) > 0 {
			fmt.Println(`WARNING! The following external containers will be purged:`)
			// print first 12 characters of ID and first configured name alias
			for _, externalCtn := range listCtn {
				fmt.Printf("	- %s (%s)\n", externalCtn.ID[0:12], externalCtn.Names[0])
			}
		}
		fmt.Print(`Are you sure you want to continue? [y/N] `)
		answer, err := reader.ReadString('\n')
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}
		if strings.ToLower(answer)[0] != 'y' {
			os.Exit(0)
		}
	}

	// Purge all the external containers with storage
	registry.ContainerEngine().ContainerRm(registry.Context(), listCtnIds, entities.RmOptions{Force: true, All: true, Ignore: true, Volumes: true})
	// Shutdown all running engines, `reset` will hijack repository
	registry.ContainerEngine().Shutdown(registry.Context())
	registry.ImageEngine().Shutdown(registry.Context())

	engine, err := infra.NewSystemEngine(entities.ResetMode, registry.PodmanConfig())
	if err != nil {
		logrus.Error(err)
		os.Exit(define.ExecErrorCodeGeneric)
	}
	defer engine.Shutdown(registry.Context())

	if err := engine.Reset(registry.Context()); err != nil {
		logrus.Error(err)
		os.Exit(define.ExecErrorCodeGeneric)
	}
	os.Exit(0)
}
