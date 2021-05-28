package system

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"text/template"
	"time"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v3/cmd/podman/parse"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/cmd/podman/validate"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/docker/go-units"
	"github.com/spf13/cobra"
)

var (
	dfSystemDescription = `
	podman system df

	Show podman disk usage
	`
	dfSystemCommand = &cobra.Command{
		Use:               "df [options]",
		Args:              validate.NoArgs,
		Short:             "Show podman disk usage",
		Long:              dfSystemDescription,
		RunE:              df,
		ValidArgsFunction: completion.AutocompleteNone,
	}
)

var (
	dfOptions entities.SystemDfOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: dfSystemCommand,
		Parent:  systemCmd,
	})
	flags := dfSystemCommand.Flags()
	flags.BoolVarP(&dfOptions.Verbose, "verbose", "v", false, "Show detailed information on disk usage")

	formatFlagName := "format"
	flags.StringVar(&dfOptions.Format, formatFlagName, "", "Pretty-print images using a Go template")
	_ = dfSystemCommand.RegisterFlagCompletionFunc(formatFlagName, completion.AutocompleteNone)
}

func df(cmd *cobra.Command, args []string) error {
	reports, err := registry.ContainerEngine().SystemDf(registry.Context(), dfOptions)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 8, 2, 2, ' ', 0)

	if dfOptions.Verbose {
		return printVerbose(w, cmd, reports)
	}
	return printSummary(w, cmd, reports)
}

func printSummary(w *tabwriter.Writer, cmd *cobra.Command, reports *entities.SystemDfReport) error {
	var (
		dfSummaries       []*dfSummary
		active            int
		size, reclaimable int64
	)
	for _, i := range reports.Images {
		if i.Containers > 0 {
			active++
		}
		size += i.Size
		if i.Containers < 1 {
			reclaimable += i.Size
		}
	}
	imageSummary := dfSummary{
		Type:        "Images",
		Total:       len(reports.Images),
		Active:      active,
		size:        size,
		reclaimable: reclaimable,
	}
	dfSummaries = append(dfSummaries, &imageSummary)

	// Containers
	var (
		conActive               int
		conSize, conReclaimable int64
	)
	for _, c := range reports.Containers {
		if c.Status == "running" {
			conActive++
		} else {
			conReclaimable += c.RWSize
		}
		conSize += c.RWSize
	}
	containerSummary := dfSummary{
		Type:        "Containers",
		Total:       len(reports.Containers),
		Active:      conActive,
		size:        conSize,
		reclaimable: conReclaimable,
	}
	dfSummaries = append(dfSummaries, &containerSummary)

	// Volumes
	var (
		activeVolumes                   int
		volumesSize, volumesReclaimable int64
	)

	for _, v := range reports.Volumes {
		activeVolumes += v.Links
		volumesSize += v.Size
		volumesReclaimable += v.ReclaimableSize
	}
	volumeSummary := dfSummary{
		Type:        "Local Volumes",
		Total:       len(reports.Volumes),
		Active:      activeVolumes,
		size:        volumesSize,
		reclaimable: volumesReclaimable,
	}
	dfSummaries = append(dfSummaries, &volumeSummary)

	// need to give un-exported fields
	hdrs := report.Headers(dfSummary{}, map[string]string{
		"Size":        "SIZE",
		"Reclaimable": "RECLAIMABLE",
	})
	row := "{{.Type}}\t{{.Total}}\t{{.Active}}\t{{.Size}}\t{{.Reclaimable}}\n"
	if cmd.Flags().Changed("format") {
		row = report.NormalizeFormat(dfOptions.Format)
	}
	return writeTemplate(w, cmd, hdrs, row, dfSummaries)
}

func printVerbose(w *tabwriter.Writer, cmd *cobra.Command, reports *entities.SystemDfReport) error {
	defer w.Flush()

	fmt.Fprint(w, "Images space usage:\n\n")
	// convert to dfImage for output
	dfImages := make([]*dfImage, 0, len(reports.Images))
	for _, d := range reports.Images {
		dfImages = append(dfImages, &dfImage{SystemDfImageReport: d})
	}
	hdrs := report.Headers(entities.SystemDfImageReport{}, map[string]string{
		"ImageID":    "IMAGE ID",
		"SharedSize": "SHARED SIZE",
		"UniqueSize": "UNIQUE SIZE",
	})
	imageRow := "{{.Repository}}\t{{.Tag}}\t{{.ImageID}}\t{{.Created}}\t{{.Size}}\t{{.SharedSize}}\t{{.UniqueSize}}\t{{.Containers}}\n"
	if err := writeTemplate(w, cmd, hdrs, imageRow, dfImages); err != nil {
		return nil
	}

	fmt.Fprint(w, "\nContainers space usage:\n\n")
	// convert to dfContainers for output
	dfContainers := make([]*dfContainer, 0, len(reports.Containers))
	for _, d := range reports.Containers {
		dfContainers = append(dfContainers, &dfContainer{SystemDfContainerReport: d})
	}
	hdrs = report.Headers(entities.SystemDfContainerReport{}, map[string]string{
		"ContainerID":  "CONTAINER ID",
		"LocalVolumes": "LOCAL VOLUMES",
		"RWSize":       "SIZE",
	})
	containerRow := "{{.ContainerID}}\t{{.Image}}\t{{.Command}}\t{{.LocalVolumes}}\t{{.RWSize}}\t{{.Created}}\t{{.Status}}\t{{.Names}}\n"
	if err := writeTemplate(w, cmd, hdrs, containerRow, dfContainers); err != nil {
		return nil
	}

	fmt.Fprint(w, "\nLocal Volumes space usage:\n\n")
	dfVolumes := make([]*dfVolume, 0, len(reports.Volumes))
	// convert to dfVolume for output
	for _, d := range reports.Volumes {
		dfVolumes = append(dfVolumes, &dfVolume{SystemDfVolumeReport: d})
	}
	hdrs = report.Headers(entities.SystemDfVolumeReport{}, map[string]string{
		"VolumeName": "VOLUME NAME",
	})
	volumeRow := "{{.VolumeName}}\t{{.Links}}\t{{.Size}}\n"
	return writeTemplate(w, cmd, hdrs, volumeRow, dfVolumes)
}

func writeTemplate(w *tabwriter.Writer, cmd *cobra.Command, hdrs []map[string]string, format string, output interface{}) error {
	defer w.Flush()

	format = parse.EnforceRange(format)
	tmpl, err := template.New("df").Parse(format)
	if err != nil {
		return err
	}

	if !cmd.Flags().Changed("format") {
		if err := tmpl.Execute(w, hdrs); err != nil {
			return err
		}
	}
	return tmpl.Execute(w, output)
}

type dfImage struct {
	*entities.SystemDfImageReport
}

func (d *dfImage) ImageID() string {
	return d.SystemDfImageReport.ImageID[0:12]
}

func (d *dfImage) Created() string {
	return units.HumanDuration(time.Since(d.SystemDfImageReport.Created))
}

func (d *dfImage) Size() string {
	return units.HumanSize(float64(d.SystemDfImageReport.Size))
}

func (d *dfImage) SharedSize() string {
	return units.HumanSize(float64(d.SystemDfImageReport.SharedSize))
}

func (d *dfImage) UniqueSize() string {
	return units.HumanSize(float64(d.SystemDfImageReport.UniqueSize))
}

type dfContainer struct {
	*entities.SystemDfContainerReport
}

func (d *dfContainer) ContainerID() string {
	return d.SystemDfContainerReport.ContainerID[0:12]
}

func (d *dfContainer) Image() string {
	return d.SystemDfContainerReport.Image[0:12]
}

func (d *dfContainer) Command() string {
	return strings.Join(d.SystemDfContainerReport.Command, " ")
}

func (d *dfContainer) RWSize() string {
	return units.HumanSize(float64(d.SystemDfContainerReport.RWSize))
}

func (d *dfContainer) Created() string {
	return units.HumanDuration(time.Since(d.SystemDfContainerReport.Created))
}

type dfVolume struct {
	*entities.SystemDfVolumeReport
}

func (d *dfVolume) Size() string {
	return units.HumanSize(float64(d.SystemDfVolumeReport.Size))
}

type dfSummary struct {
	Type        string
	Total       int
	Active      int
	size        int64
	reclaimable int64
}

func (d *dfSummary) Size() string {
	return units.HumanSize(float64(d.size))
}

func (d *dfSummary) Reclaimable() string {
	percent := int(float64(d.reclaimable)/float64(d.size)) * 100
	return fmt.Sprintf("%s (%d%%)", units.HumanSize(float64(d.reclaimable)), percent)
}
