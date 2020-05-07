package containers

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"text/template"

	tm "github.com/buger/goterm"
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/cgroups"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/utils"
	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	statsDescription = "Display percentage of CPU, memory, network I/O, block I/O and PIDs for one or more containers."
	statsCommand     = &cobra.Command{
		Use:   "stats [flags] [CONTAINER...]",
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

var (
	statsOptions       entities.ContainerStatsOptions
	defaultStatsRow    = "{{.ID}}\t{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}\t{{.NetIO}}\t{{.BlockIO}}\t{{.PIDS}}\n"
	defaultStatsHeader = "ID\tNAME\tCPU %\tMEM USAGE / LIMIT\tMEM %\tNET IO\tBLOCK IO\tPIDS\n"
)

func statFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&statsOptions.All, "all", "a", false, "Show all containers. Only running containers are shown by default. The default is false")
	flags.StringVar(&statsOptions.Format, "format", "", "Pretty-print container statistics to JSON or using a Go template")
	flags.BoolVarP(&statsOptions.Latest, "latest", "l", false, "Act on the latest container Podman is aware of")
	flags.BoolVar(&statsOptions.NoReset, "no-reset", false, "Disable resetting the screen between intervals")
	flags.BoolVar(&statsOptions.NoStream, "no-stream", false, "Disable streaming stats and only pull the first result, default setting is false")
	if registry.IsRemote() {
		_ = flags.MarkHidden("latest")
	}
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: statsCommand,
	})
	flags := statsCommand.Flags()
	statFlags(flags)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: containerStatsCommand,
		Parent:  containerCmd,
	})

	containerStatsFlags := containerStatsCommand.Flags()
	statFlags(containerStatsFlags)
}

// stats is different in that it will assume running containers if
// no input is given, so we need to validate differently
func checkStatOptions(cmd *cobra.Command, args []string) error {
	opts := 0
	if statsOptions.All {
		opts += 1
	}
	if statsOptions.Latest {
		opts += 1
	}
	if len(args) > 0 {
		opts += 1
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
	statsOptions.StatChan = make(chan []*define.ContainerStats, 1)
	go func() {
		for reports := range statsOptions.StatChan {
			if err := outputStats(reports); err != nil {
				logrus.Error(err)
			}
		}
	}()
	return registry.ContainerEngine().ContainerStats(registry.Context(), args, statsOptions)
}

func outputStats(reports []*define.ContainerStats) error {
	if len(statsOptions.Format) < 1 && !statsOptions.NoReset {
		tm.Clear()
		tm.MoveCursor(1, 1)
		tm.Flush()
	}
	var stats []*containerStats
	for _, r := range reports {
		stats = append(stats, &containerStats{r})
	}
	if statsOptions.Format == "json" {
		return outputJSON(stats)
	}
	format := defaultStatsRow
	if len(statsOptions.Format) > 0 {
		format = statsOptions.Format
		if !strings.HasSuffix(format, "\n") {
			format += "\n"
		}
	}
	format = "{{range . }}" + format + "{{end}}"
	if len(statsOptions.Format) < 1 {
		format = defaultStatsHeader + format
	}
	tmpl, err := template.New("stats").Parse(format)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 8, 2, 2, ' ', 0)
	if err := tmpl.Execute(w, stats); err != nil {
		return err
	}
	if err := w.Flush(); err != nil {
		return err
	}
	return nil
}

type containerStats struct {
	*define.ContainerStats
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

func outputJSON(stats []*containerStats) error {
	type jstat struct {
		Id         string `json:"id"`
		Name       string `json:"name"`
		CpuPercent string `json:"cpu_percent"`
		MemUsage   string `json:"mem_usage"`
		MemPerc    string `json:"mem_percent"`
		NetIO      string `json:"net_io"`
		BlockIO    string `json:"block_io"`
		Pids       string `json:"pids"`
	}
	var jstats []jstat
	for _, j := range stats {
		jstats = append(jstats, jstat{
			Id:         j.ID(),
			Name:       j.Name,
			CpuPercent: j.CPUPerc(),
			MemUsage:   j.MemPerc(),
			MemPerc:    j.MemUsage(),
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
