package system

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/pkg/domain/entities"
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

	if dfOptions.Verbose {
		return printVerbose(cmd, reports)
	}
	return printSummary(cmd, reports)
}

func printSummary(cmd *cobra.Command, reports *entities.SystemDfReport) error {
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

	rpt := report.New(os.Stdout, cmd.Name())
	defer rpt.Flush()

	var err error
	if cmd.Flags().Changed("format") {
		rpt, err = rpt.Parse(report.OriginUser, dfOptions.Format)
	} else {
		row := "{{range . }}{{.Type}}\t{{.Total}}\t{{.Active}}\t{{.Size}}\t{{.Reclaimable}}\n{{end -}}"
		rpt, err = rpt.Parse(report.OriginPodman, row)
	}
	if err != nil {
		return err
	}
	return writeTemplate(rpt, hdrs, dfSummaries)
}

func printVerbose(cmd *cobra.Command, reports *entities.SystemDfReport) error { // nolint:interfacer
	rpt := report.New(os.Stdout, cmd.Name())
	defer rpt.Flush()

	fmt.Fprint(rpt.Writer(), "Images space usage:\n\n")
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
	imageRow := "{{range .}}{{.Repository}}\t{{.Tag}}\t{{.ImageID}}\t{{.Created}}\t{{.Size}}\t{{.SharedSize}}\t{{.UniqueSize}}\t{{.Containers}}\n{{end -}}"
	rpt, err := rpt.Parse(report.OriginPodman, imageRow)
	if err != nil {
		return err
	}
	if err := writeTemplate(rpt, hdrs, dfImages); err != nil {
		return nil
	}

	fmt.Fprint(rpt.Writer(), "\nContainers space usage:\n\n")
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
	containerRow := "{{range .}}{{.ContainerID}}\t{{.Image}}\t{{.Command}}\t{{.LocalVolumes}}\t{{.RWSize}}\t{{.Created}}\t{{.Status}}\t{{.Names}}\n{{end -}}"
	rpt, err = rpt.Parse(report.OriginPodman, containerRow)
	if err != nil {
		return err
	}
	if err := writeTemplate(rpt, hdrs, dfContainers); err != nil {
		return nil
	}

	fmt.Fprint(rpt.Writer(), "\nLocal Volumes space usage:\n\n")
	dfVolumes := make([]*dfVolume, 0, len(reports.Volumes))
	// convert to dfVolume for output
	for _, d := range reports.Volumes {
		dfVolumes = append(dfVolumes, &dfVolume{SystemDfVolumeReport: d})
	}
	hdrs = report.Headers(entities.SystemDfVolumeReport{}, map[string]string{
		"VolumeName": "VOLUME NAME",
	})
	volumeRow := "{{range .}}{{.VolumeName}}\t{{.Links}}\t{{.Size}}\n{{end -}}"
	rpt, err = rpt.Parse(report.OriginPodman, volumeRow)
	if err != nil {
		return err
	}
	return writeTemplate(rpt, hdrs, dfVolumes)
}

func writeTemplate(rpt *report.Formatter, hdrs []map[string]string, output interface{}) error {
	if rpt.RenderHeaders {
		if err := rpt.Execute(hdrs); err != nil {
			return err
		}
	}
	return rpt.Execute(output)
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
