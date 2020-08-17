package pods

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"text/tabwriter"
	"text/template"
	"time"

	"github.com/buger/goterm"
	"github.com/containers/buildah/pkg/formats"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/validate"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/util/camelcase"
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
		Use:   "stats [flags] [POD...]",
		Short: "Display a live stream of resource usage statistics for the containers in one or more pods",
		Long:  statsDescription,
		RunE:  stats,
		Example: `podman pod stats
  podman pod stats a69b23034235 named-pod
  podman pod stats --latest
  podman pod stats --all`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: statsCmd,
		Parent:  podCmd,
	})

	flags := statsCmd.Flags()
	flags.BoolVarP(&statsOptions.All, "all", "a", false, "Provide stats for all pods")
	flags.StringVar(&statsOptions.Format, "format", "", "Pretty-print container statistics to JSON or using a Go template")
	flags.BoolVar(&statsOptions.NoReset, "no-reset", false, "Disable resetting the screen when streaming")
	flags.BoolVar(&statsOptions.NoStream, "no-stream", false, "Disable streaming stats and only pull the first result")
	validate.AddLatestFlag(statsCmd, &statsOptions.Latest)
}

func stats(cmd *cobra.Command, args []string) error {
	// Validate input.
	if err := entities.ValidatePodStatsOptions(args, &statsOptions.PodStatsOptions); err != nil {
		return err
	}

	format := statsOptions.Format
	doJSON := strings.ToLower(format) == formats.JSONString
	header := getPodStatsHeader(format)

	for {
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
			if len(format) == 0 {
				printPodStatsLines(reports)
			} else if err := printFormattedPodStatsLines(format, reports, header); err != nil {
				return err
			}
		}
		if statsOptions.NoStream {
			break
		}
		time.Sleep(time.Second)
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
	for _, i := range stats {
		if len(stats) == 0 {
			fmt.Fprintf(w, outFormat, i.Pod, "--", "--", "--", "--", "--", "--", "--", "--")
		} else {
			fmt.Fprintf(w, outFormat, i.Pod, i.CID, i.Name, i.CPU, i.MemUsage, i.Mem, i.NetIO, i.BlockIO, i.PIDS)
		}
	}
	w.Flush()
}

func printFormattedPodStatsLines(format string, stats []*entities.PodStatsReport, headerNames map[string]string) error {
	if len(stats) == 0 {
		return nil
	}

	// Use a tabwriter to align column format
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	// Spit out the header if "table" is present in the format
	if strings.HasPrefix(format, "table") {
		hformat := strings.Replace(strings.TrimSpace(format[5:]), " ", "\t", -1)
		format = hformat
		headerTmpl, err := template.New("header").Parse(hformat)
		if err != nil {
			return err
		}
		if err := headerTmpl.Execute(w, headerNames); err != nil {
			return err
		}
		fmt.Fprintln(w, "")
	}

	// Spit out the data rows now
	dataTmpl, err := template.New("data").Parse(format)
	if err != nil {
		return err
	}
	for _, s := range stats {
		if err := dataTmpl.Execute(w, s); err != nil {
			return err
		}
		fmt.Fprintln(w, "")
	}
	// Flush the writer
	return w.Flush()

}

// getPodStatsHeader returns the stats header for the specified options.
func getPodStatsHeader(format string) map[string]string {
	headerNames := make(map[string]string)
	if format == "" {
		return headerNames
	}
	// Make a map of the field names for the headers
	v := reflect.ValueOf(entities.PodStatsReport{})
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		split := camelcase.Split(t.Field(i).Name)
		value := strings.ToUpper(strings.Join(split, " "))
		switch value {
		case "CPU", "MEM":
			value += " %"
		case "MEM USAGE":
			value = "MEM USAGE / LIMIT"
		}
		headerNames[t.Field(i).Name] = value
	}
	return headerNames
}
