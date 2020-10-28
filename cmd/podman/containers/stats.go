package containers

import (
	"fmt"
	"os"
	"text/tabwriter"
	"text/template"

	tm "github.com/buger/goterm"
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v2/cmd/podman/parse"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/validate"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/cgroups"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/rootless"
	"github.com/containers/podman/v2/utils"
	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	statsDescription = "Display percentage of CPU, memory, network I/O, block I/O and PIDs for one or more containers."
	statsCommand     = &cobra.Command{
		Use:   "stats [options] [CONTAINER...]",
		Short: "Display a live stream of container resource usage statistics",
		Long:  statsDescription,
		RunE:  stats,
		Args:  checkStatOptions,
		Example: `podman stats --all --no-stream
  podman stats ctrID
  podman stats --no-stream --format "table {{.ID}} {{.Name}} {{.MemUsage}}" ctrID`,
	}

	containerStatsCommand = &cobra.Command{
		Use:   statsCommand.Use,
		Short: statsCommand.Short,
		Long:  statsCommand.Long,
		RunE:  statsCommand.RunE,
		Args:  checkStatOptions,
		Example: `podman container stats --all --no-stream
  podman container stats ctrID
  podman container stats --no-stream --format "table {{.ID}} {{.Name}} {{.MemUsage}}" ctrID`,
	}
)

// statsOptionsCLI is used for storing CLI arguments. Some fields are later
// used in the backend.
type statsOptionsCLI struct {
	All      bool
	Format   string
	Latest   bool
	NoReset  bool
	NoStream bool
}

var (
	statsOptions statsOptionsCLI
)

func statFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&statsOptions.All, "all", "a", false, "Show all containers. Only running containers are shown by default. The default is false")
	flags.StringVar(&statsOptions.Format, "format", "", "Pretty-print container statistics to JSON or using a Go template")
	flags.BoolVar(&statsOptions.NoReset, "no-reset", false, "Disable resetting the screen between intervals")
	flags.BoolVar(&statsOptions.NoStream, "no-stream", false, "Disable streaming stats and only pull the first result, default setting is false")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: statsCommand,
	})
	statFlags(statsCommand.Flags())
	validate.AddLatestFlag(statsCommand, &statsOptions.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: containerStatsCommand,
		Parent:  containerCmd,
	})
	statFlags(containerStatsCommand.Flags())
	validate.AddLatestFlag(containerStatsCommand, &statsOptions.Latest)
}

// stats is different in that it will assume running containers if
// no input is given, so we need to validate differently
func checkStatOptions(cmd *cobra.Command, args []string) error {
	opts := 0
	if statsOptions.All {
		opts++
	}
	if statsOptions.Latest {
		opts++
	}
	if len(args) > 0 {
		opts++
	}
	if opts > 1 {
		return errors.Errorf("--all, --latest and containers cannot be used together")
	}
	return nil
}

func stats(cmd *cobra.Command, args []string) error {
	if rootless.IsRootless() {
		unified, err := cgroups.IsCgroup2UnifiedMode()
		if err != nil {
			return err
		}
		if !unified {
			return errors.New("stats is not supported in rootless mode without cgroups v2")
		}
	}

	// Convert to the entities options.  We should not leak CLI-only
	// options into the backend and separate concerns.
	opts := entities.ContainerStatsOptions{
		Latest: statsOptions.Latest,
		Stream: !statsOptions.NoStream,
	}
	statsChan, err := registry.ContainerEngine().ContainerStats(registry.Context(), args, opts)
	if err != nil {
		return err
	}
	for report := range statsChan {
		if report.Error != nil {
			return report.Error
		}
		if err := outputStats(report.Stats); err != nil {
			logrus.Error(err)
		}
	}
	return nil
}

func outputStats(reports []define.ContainerStats) error {
	headers := report.Headers(define.ContainerStats{}, map[string]string{
		"ID":       "ID",
		"CPUPerc":  "CPU %",
		"MemUsage": "MEM USAGE / LIMIT",
		"MemPerc":  "MEM %",
		"NetIO":    "NET IO",
		"BlockIO":  "BLOCK IO",
		"PIDS":     "PIDS",
	})
	if !statsOptions.NoReset {
		tm.Clear()
		tm.MoveCursor(1, 1)
		tm.Flush()
	}
	stats := make([]containerStats, 0, len(reports))
	for _, r := range reports {
		stats = append(stats, containerStats{r})
	}
	if report.IsJSON(statsOptions.Format) {
		return outputJSON(stats)
	}
	format := "{{.ID}}\t{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}\t{{.NetIO}}\t{{.BlockIO}}\t{{.PIDS}}\n"
	if len(statsOptions.Format) > 0 {
		format = report.NormalizeFormat(statsOptions.Format)
	}
	format = parse.EnforceRange(format)

	tmpl, err := template.New("stats").Parse(format)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 8, 2, 2, ' ', 0)
	defer w.Flush()

	if len(statsOptions.Format) < 1 {
		if err := tmpl.Execute(w, headers); err != nil {
			return err
		}
	}
	if err := tmpl.Execute(w, stats); err != nil {
		return err
	}
	return nil
}

type containerStats struct {
	define.ContainerStats
}

func (s *containerStats) ID() string {
	return s.ContainerID[0:12]
}

func (s *containerStats) CPUPerc() string {
	return floatToPercentString(s.CPU)
}

func (s *containerStats) MemPerc() string {
	return floatToPercentString(s.ContainerStats.MemPerc)
}

func (s *containerStats) NetIO() string {
	return combineHumanValues(s.NetInput, s.NetOutput)
}

func (s *containerStats) BlockIO() string {
	return combineHumanValues(s.BlockInput, s.BlockOutput)
}

func (s *containerStats) PIDS() string {
	if s.PIDs == 0 {
		// If things go bazinga, return a safe value
		return "--"
	}
	return fmt.Sprintf("%d", s.PIDs)
}
func (s *containerStats) MemUsage() string {
	return combineHumanValues(s.ContainerStats.MemUsage, s.ContainerStats.MemLimit)
}

func floatToPercentString(f float64) string {
	strippedFloat, err := utils.RemoveScientificNotationFromFloat(f)
	if err != nil || strippedFloat == 0 {
		// If things go bazinga, return a safe value
		return "--"
	}
	return fmt.Sprintf("%.2f", strippedFloat) + "%"
}

func combineHumanValues(a, b uint64) string {
	if a == 0 && b == 0 {
		return "-- / --"
	}
	return fmt.Sprintf("%s / %s", units.HumanSize(float64(a)), units.HumanSize(float64(b)))
}

func outputJSON(stats []containerStats) error {
	type jstat struct {
		Id         string `json:"id"` // nolint
		Name       string `json:"name"`
		CpuPercent string `json:"cpu_percent"` // nolint
		MemUsage   string `json:"mem_usage"`
		MemPerc    string `json:"mem_percent"`
		NetIO      string `json:"net_io"`
		BlockIO    string `json:"block_io"`
		Pids       string `json:"pids"`
	}
	jstats := make([]jstat, 0, len(stats))
	for _, j := range stats {
		jstats = append(jstats, jstat{
			Id:         j.ID(),
			Name:       j.Name,
			CpuPercent: j.CPUPerc(),
			MemUsage:   j.MemUsage(),
			MemPerc:    j.MemPerc(),
			NetIO:      j.NetIO(),
			BlockIO:    j.BlockIO(),
			Pids:       j.PIDS(),
		})
	}
	b, err := json.MarshalIndent(jstats, "", " ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
