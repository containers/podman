package pods

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/cmd/podmanV2/report"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	psDescription = "List all pods on system including their names, ids and current state."

	// Command: podman pod _ps_
	psCmd = &cobra.Command{
		Use:     "ps",
		Aliases: []string{"ls", "list"},
		Short:   "list pods",
		Long:    psDescription,
		RunE:    pods,
	}
)

var (
	defaultHeaders string = "POD ID\tNAME\tSTATUS\tCREATED"
	inputFilters   string
	noTrunc        bool
	psInput        entities.PodPSOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: psCmd,
		Parent:  podCmd,
	})
	flags := psCmd.Flags()
	flags.BoolVar(&psInput.CtrNames, "ctr-names", false, "Display the container names")
	flags.BoolVar(&psInput.CtrIds, "ctr-ids", false, "Display the container UUIDs. If no-trunc is not set they will be truncated")
	flags.BoolVar(&psInput.CtrStatus, "ctr-status", false, "Display the container status")
	// TODO should we make this a [] ?
	flags.StringVarP(&inputFilters, "filter", "f", "", "Filter output based on conditions given")
	flags.StringVar(&psInput.Format, "format", "", "Pretty-print pods to JSON or using a Go template")
	flags.BoolVarP(&psInput.Latest, "latest", "l", false, "Act on the latest pod podman is aware of")
	flags.BoolVar(&psInput.Namespace, "namespace", false, "Display namespace information of the pod")
	flags.BoolVar(&psInput.Namespace, "ns", false, "Display namespace information of the pod")
	flags.BoolVar(&noTrunc, "no-trunc", false, "Do not truncate pod and container IDs")
	flags.BoolVarP(&psInput.Quiet, "quiet", "q", false, "Print the numeric IDs of the pods only")
	flags.StringVar(&psInput.Sort, "sort", "created", "Sort output by created, id, name, or number")
	if registry.IsRemote() {
		_ = flags.MarkHidden("latest")
	}
}

func pods(cmd *cobra.Command, args []string) error {
	var (
		w   io.Writer = os.Stdout
		row string
	)
	if cmd.Flag("filter").Changed {
		for _, f := range strings.Split(inputFilters, ",") {
			split := strings.Split(f, "=")
			if len(split) < 2 {
				return errors.Errorf("filter input must be in the form of filter=value: %s is invalid", f)
			}
			psInput.Filters[split[0]] = append(psInput.Filters[split[0]], split[1])
		}
	}
	responses, err := registry.ContainerEngine().PodPs(context.Background(), psInput)
	if err != nil {
		return err
	}

	if psInput.Format == "json" {
		b, err := json.MarshalIndent(responses, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}
	headers, row := createPodPsOut()
	if psInput.Quiet {
		if noTrunc {
			row = "{{.Id}}\n"
		} else {
			row = "{{slice .Id 0 12}}\n"
		}
	}
	if cmd.Flag("format").Changed {
		row = psInput.Format
		if !strings.HasPrefix(row, "\n") {
			row += "\n"
		}
	}
	format := "{{range . }}" + row + "{{end}}"
	if !psInput.Quiet && !cmd.Flag("format").Changed {
		format = headers + format
	}
	funcs := report.AppendFuncMap(podFuncMap)
	tmpl, err := template.New("listPods").Funcs(funcs).Parse(format)
	if err != nil {
		return err
	}
	if !psInput.Quiet {
		w = tabwriter.NewWriter(os.Stdout, 8, 2, 2, ' ', 0)
	}
	if err := tmpl.Execute(w, responses); err != nil {
		return err
	}
	if flusher, ok := w.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}

func createPodPsOut() (string, string) {
	var row string
	headers := defaultHeaders
	if noTrunc {
		row += "{{.Id}}"
	} else {
		row += "{{slice .Id 0 12}}"
	}

	row += "\t{{.Name}}\t{{.Status}}\t{{humanDurationFromTime .Created}}"

	//rowFormat      string = "{{slice .Id 0 12}}\t{{.Name}}\t{{.Status}}\t{{humanDurationFromTime .Created}}"
	if psInput.CtrIds {
		headers += "\tIDS"
		row += "\t{{podcids .Containers}}"
	}
	if psInput.CtrNames {
		headers += "\tNAMES"
		row += "\t{{podconnames .Containers}}"
	}
	if psInput.CtrStatus {
		headers += "\tSTATUS"
		row += "\t{{podconstatuses .Containers}}"
	}
	if psInput.Namespace {
		headers += "\tCGROUP\tNAMESPACES"
		row += "\t{{.Cgroup}}\t{{.Namespace}}"
	}
	if !psInput.CtrStatus && !psInput.CtrNames && !psInput.CtrIds {
		headers += "\t# OF CONTAINERS"
		row += "\t{{numCons .Containers}}"

	}
	headers += "\tINFRA ID\n"
	if noTrunc {
		row += "\t{{.InfraId}}\n"
	} else {
		row += "\t{{slice .InfraId 0 12}}\n"
	}
	return headers, row
}
