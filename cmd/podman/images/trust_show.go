package images

import (
	"fmt"
	"os"
	"text/tabwriter"
	"text/template"

	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	showTrustDescription = "Display trust policy for the system"
	showTrustCommand     = &cobra.Command{
		Annotations:       map[string]string{registry.EngineMode: registry.ABIMode},
		Use:               "show [options] [REGISTRY]",
		Short:             "Display trust policy for the system",
		Long:              showTrustDescription,
		RunE:              showTrust,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: common.AutocompleteRegistries,
		Example:           "",
	}
)

var (
	showTrustOptions entities.ShowTrustOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: showTrustCommand,
		Parent:  trustCmd,
	})
	showFlags := showTrustCommand.Flags()
	showFlags.BoolVarP(&showTrustOptions.JSON, "json", "j", false, "Output as json")
	showFlags.StringVar(&showTrustOptions.PolicyPath, "policypath", "", "")
	showFlags.BoolVar(&showTrustOptions.Raw, "raw", false, "Output raw policy file")
	_ = showFlags.MarkHidden("policypath")
	showFlags.StringVar(&showTrustOptions.RegistryPath, "registrypath", "", "")
	_ = showFlags.MarkHidden("registrypath")
}

func showTrust(cmd *cobra.Command, args []string) error {
	report, err := registry.ImageEngine().ShowTrust(registry.Context(), args, showTrustOptions)
	if err != nil {
		return err
	}
	if showTrustOptions.Raw {
		fmt.Println(string(report.Raw))
		return nil
	}
	if showTrustOptions.JSON {
		b, err := json.MarshalIndent(report.Policies, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}

	row := "{{.RepoName}}\t{{.Type}}\t{{.GPGId}}\t{{.SignatureStore}}\n"
	format := "{{range . }}" + row + "{{end}}"
	tmpl, err := template.New("listContainers").Parse(format)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 8, 2, 2, ' ', 0)
	if err := tmpl.Execute(w, report.Policies); err != nil {
		return err
	}
	if err := w.Flush(); err != nil {
		return err
	}
	return nil
}
