package quadlet

import (
	"errors"
	"fmt"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	quadletInstallDescription = `Install one or more Quadlets for the current user. Quadlets may be specified as local files, Web URLs, and OCI artifacts.`

	quadletInstallCmd = &cobra.Command{
		Use:   "install [options] PATH-OR-URL [PATH-OR-URL...]",
		Short: "Install one or more quadlets",
		Long:  quadletInstallDescription,
		RunE:  install,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("must provide at least one argument")
			}
			return nil
		},
		// TODO: Autocomplete valid extensions only
		ValidArgsFunction: completion.AutocompleteDefault,
		Example: `podman quadlet install /path/to/myquadlet.container
podman quadlet install https://github.com/containers/podman/blob/main/test/e2e/quadlet/basic.container
podman quadlet install oci-artifact://my-artifact:latest`,
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
	installReport, err := registry.ContainerEngine().QuadletInstall(registry.Context(), args, installOptions)
	if err != nil {
		return err
	}
	for pathOrURL, err := range installReport.QuadletErrors {
		logrus.Errorf("Quadlet %q failed to install: %v", pathOrURL, err)
	}
	for _, s := range installReport.InstalledQuadlets {
		fmt.Println(s)
	}

	if len(installReport.QuadletErrors) > 0 {
		return errors.New("errors occurred installing some Quadlets")
	}

	return nil
}
