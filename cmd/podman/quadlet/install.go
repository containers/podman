package quadlet

import (
	"errors"
	"fmt"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/utils"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	quadletInstallDescription = `Install a quadlet file or quadlet application. Quadlets may be specified as local files, Web URLs, and OCI artifacts.`

	quadletInstallCmd = &cobra.Command{
		Use:   "install [options] QUADLET-PATH-OR-URL [FILES-PATH-OR-URL...]",
		Short: "Install a quadlet file or quadlet application",
		Long:  quadletInstallDescription,
		RunE:  install,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("must provide at least one argument")
			}
			return nil
		},
		ValidArgsFunction: completion.AutocompleteDefault,
		Example: `podman quadlet install /path/to/myquadlet.container
podman quadlet install https://github.com/containers/podman/blob/main/test/e2e/quadlet/basic.container`,
	}

	installOptions entities.QuadletInstallOptions
)

func installFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.BoolVar(&installOptions.ReloadSystemd, "reload-systemd", true, "Reload systemd after installing Quadlets")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: quadletInstallCmd,
		Parent:  quadletCmd,
	})
	installFlags(quadletInstallCmd)
}

func install(cmd *cobra.Command, args []string) error {
	var errs utils.OutputErrors
	installReport, err := registry.ContainerEngine().QuadletInstall(registry.Context(), args, installOptions)
	if err != nil {
		return err
	}
	for pathOrURL, err := range installReport.QuadletErrors {
		errs = append(errs, fmt.Errorf("quadlet %q failed to install: %v", pathOrURL, err))
	}
	for _, s := range installReport.InstalledQuadlets {
		fmt.Println(s)
	}

	if len(installReport.QuadletErrors) > 0 {
		errs = append(errs, errors.New("errors occurred installing some Quadlets"))
	}

	return errs.PrintErrors()
}
