package pods

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"text/template"
	"time"

	"github.com/buger/goterm"
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/parse"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/cmd/podman/validate"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/spf13/cobra"
)

type podStatsOptionsWrapper struct {
	entities.PodStatsOptions

	// Format - pretty-print to JSON or a go template.
	Format string
	// NoReset - do not reset the screen when streaming.
	NoReset bool
	// NoStream - do not stream stats but write them once.
	NoStream bool
}

var (
	statsOptions     = podStatsOptionsWrapper{}
	statsDescription = `Display the containers' resource-usage statistics of one or more running pod`
	// Command: podman pod _pod_
	statsCmd = &cobra.Command{
		Use:               "stats [options] [POD...]",
		Short:             "Display a live stream of resource usage statistics for the containers in one or more pods",
		Long:              statsDescription,
		RunE:              stats,
		ValidArgsFunction: common.AutocompletePodsRunning,
		Example: `podman pod stats
  podman pod stats a69b23034235 named-pod
  podman pod stats --latest
  podman pod stats --all`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: statsCmd,
		Parent:  podCmd,
	})

	flags := statsCmd.Flags()
	flags.BoolVarP(&statsOptions.All, "all", "a", false, "Provide stats for all pods")

	formatFlagName := "format"
	flags.StringVar(&statsOptions.Format, formatFlagName, "", "Pretty-print container statistics to JSON or using a Go template")
	_ = statsCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(entities.PodStatsReport{}))

	flags.BoolVar(&statsOptions.NoReset, "no-reset", false, "Disable resetting the screen when streaming")
	flags.BoolVar(&statsOptions.NoStream, "no-stream", false, "Disable streaming stats and only pull the first result")
	validate.AddLatestFlag(statsCmd, &statsOptions.Latest)
}

func stats(cmd *cobra.Command, args []string) error {
	// Validate input.
	if err := entities.ValidatePodStatsOptions(args, &statsOptions.PodStatsOptions); err != nil {
		return err
	}

	row := report.NormalizeFormat(statsOptions.Format)
	doJSON := report.IsJSON(row)

	headers := report.Headers(entities.PodStatsReport{}, map[string]string{
		"CPU":           "CPU %",
		"MemUsage":      "MEM USAGE/ LIMIT",
		"MemUsageBytes": "MEM USAGE/ LIMIT",
		"MEM":           "MEM %",
		"NET IO":        "NET IO",
		"BlockIO":       "BLOCK IO",
	})

	for ; ; time.Sleep(time.Second) {
		reports, err := registry.ContainerEngine().PodStats(context.Background(), args, statsOptions.PodStatsOptions)
		if err != nil {
			return err
		}
		// Print the stats in the requested format and configuration.
		if doJSON {
			if err := printJSONPodStats(reports); err != nil {
				return err
			}
		} else {
			if !statsOptions.NoReset {
				goterm.Clear()
				goterm.MoveCursor(1, 1)
				goterm.Flush()
			}
			if cmd.Flags().Changed("format") {
				if err := printFormattedPodStatsLines(headers, row, reports); err != nil {
					return err
				}
			} else {
				printPodStatsLines(reports)
			}
		}
		if statsOptions.NoStream {
			break
		}
	}

	return nil
}

func printJSONPodStats(stats []*entities.PodStatsReport) error {
	b, err := json.MarshalIndent(&stats, "", "     ")
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "%s\n", string(b))
	return nil
}

func printPodStatsLines(stats []*entities.PodStatsReport) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	outFormat := "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n"
	fmt.Fprintf(w, outFormat, "POD", "CID", "NAME", "CPU %", "MEM USAGE/ LIMIT", "MEM %", "NET IO", "BLOCK IO", "PIDS")
	if len(stats) == 0 {
		fmt.Fprintf(w, outFormat, "--", "--", "--", "--", "--", "--", "--", "--", "--")
	} else {
		for _, i := range stats {
			fmt.Fprintf(w, outFormat, i.Pod, i.CID, i.Name, i.CPU, i.MemUsage, i.Mem, i.NetIO, i.BlockIO, i.PIDS)
		}
	}
	w.Flush()
}

func printFormattedPodStatsLines(headerNames []map[string]string, row string, stats []*entities.PodStatsReport) error {
	if len(stats) == 0 {
		return nil
	}

	row = parse.EnforceRange(row)

	tmpl, err := template.New("pod stats").Parse(row)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	defer w.Flush()

	if err := tmpl.Execute(w, headerNames); err != nil {
		return err
	}
	return tmpl.Execute(w, stats)
}
