package pods

import (
	"strings"
	"text/template"

	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// Command: podman _pod_
	podCmd = &cobra.Command{
		Use:               "pod",
		Short:             "Manage pods",
		Long:              "Manage pods",
		TraverseChildren:  true,
		PersistentPreRunE: preRunE,
		RunE:              registry.SubCommandExists,
	}
)

var podFuncMap = template.FuncMap{
	"numCons": func(cons []*entities.ListPodContainer) int {
		return len(cons)
	},
	"podcids": func(cons []*entities.ListPodContainer) string {
		var ctrids []string
		for _, c := range cons {
			ctrids = append(ctrids, c.Id[:12])
		}
		return strings.Join(ctrids, ",")
	},
	"podconnames": func(cons []*entities.ListPodContainer) string {
		var ctrNames []string
		for _, c := range cons {
			ctrNames = append(ctrNames, c.Names[:12])
		}
		return strings.Join(ctrNames, ",")
	},
	"podconstatuses": func(cons []*entities.ListPodContainer) string {
		var statuses []string
		for _, c := range cons {
			statuses = append(statuses, c.Status)
		}
		return strings.Join(statuses, ",")
	},
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: podCmd,
	})
	podCmd.SetHelpTemplate(registry.HelpTemplate())
	podCmd.SetUsageTemplate(registry.UsageTemplate())
}

func preRunE(cmd *cobra.Command, args []string) error {
	_, err := registry.NewContainerEngine(cmd, args)
	return err
}
