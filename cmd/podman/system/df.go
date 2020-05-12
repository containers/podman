package system

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/cmd/podman/validate"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/docker/go-units"
	"github.com/spf13/cobra"
)

var (
	dfSystemDescription = `
	podman system df

	Show podman disk usage
	`
	dfSystemCommand = &cobra.Command{
		Use:   "df",
		Args:  validate.NoArgs,
		Short: "Show podman disk usage",
		Long:  dfSystemDescription,
		RunE:  df,
	}
)

var (
	dfOptions entities.SystemDfOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: dfSystemCommand,
		Parent:  systemCmd,
	})
	flags := dfSystemCommand.Flags()
	flags.BoolVarP(&dfOptions.Verbose, "verbose", "v", false, "Show detailed information on disk usage")
	flags.StringVar(&dfOptions.Format, "format", "", "Pretty-print images using a Go template")
}

func df(cmd *cobra.Command, args []string) error {
	reports, err := registry.ContainerEngine().SystemDf(registry.Context(), dfOptions)
	if err != nil {
		return err
	}
	if dfOptions.Verbose {
		return printVerbose(reports)
	}
	return printSummary(reports, dfOptions.Format)
}

func printSummary(reports *entities.SystemDfReport, userFormat string) error {

	var (
		dfSummaries       []*dfSummary
		active            int
		size, reclaimable int64
		format            string    = "{{.Type}}\t{{.Total}}\t{{.Active}}\t{{.Size}}\t{{.Reclaimable}}\n"
		w                 io.Writer = os.Stdout
	)

	//	Images
	if len(userFormat) > 0 {
		format = userFormat
	}

	for _, i := range reports.Images {
		if i.Containers > 0 {
			active += 1
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
			conActive += 1
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
		volumesReclaimable += v.Size
	}
	volumeSummary := dfSummary{
		Type:        "Local Volumes",
		Total:       len(reports.Volumes),
		Active:      activeVolumes,
		size:        volumesSize,
		reclaimable: volumesReclaimable,
	}

	dfSummaries = append(dfSummaries, &volumeSummary)

	headers := "TYPE\tTOTAL\tACTIVE\tSIZE\tRECLAIMABLE\n"
	format = "{{range . }}" + format + "{{end}}"
	if len(userFormat) == 0 {
		format = headers + format
	}
	return writeTemplate(w, format, dfSummaries)
}

func printVerbose(reports *entities.SystemDfReport) error {
	var (
		dfImages     []*dfImage
		dfContainers []*dfContainer
		dfVolumes    []*dfVolume
		w            io.Writer = os.Stdout
	)

	// Images
	fmt.Print("\nImages space usage:\n\n")
	// convert to dfImage for output
	for _, d := range reports.Images {
		dfImages = append(dfImages, &dfImage{SystemDfImageReport: d})
	}
	imageHeaders := "REPOSITORY\tTAG\tIMAGE ID\tCREATED\tSIZE\tSHARED SIZE\tUNIQUE SIZE\tCONTAINERS\n"
	imageRow := "{{.Repository}}\t{{.Tag}}\t{{.ImageID}}\t{{.Created}}\t{{.Size}}\t{{.SharedSize}}\t{{.UniqueSize}}\t{{.Containers}}\n"
	format := imageHeaders + "{{range . }}" + imageRow + "{{end}}"
	if err := writeTemplate(w, format, dfImages); err != nil {
		return nil
	}

	// Containers
	fmt.Print("\nContainers space usage:\n\n")

	// convert to dfContainers for output
	for _, d := range reports.Containers {
		dfContainers = append(dfContainers, &dfContainer{SystemDfContainerReport: d})
	}
	containerHeaders := "CONTAINER ID\tIMAGE\tCOMMAND\tLOCAL VOLUMES\tSIZE\tCREATED\tSTATUS\tNAMES\n"
	containerRow := "{{.ContainerID}}\t{{.Image}}\t{{.Command}}\t{{.LocalVolumes}}\t{{.Size}}\t{{.Created}}\t{{.Status}}\t{{.Names}}\n"
	format = containerHeaders + "{{range . }}" + containerRow + "{{end}}"
	if err := writeTemplate(w, format, dfContainers); err != nil {
		return nil
	}

	// Volumes
	fmt.Print("\nLocal Volumes space usage:\n\n")

	// convert to dfVolume for output
	for _, d := range reports.Volumes {
		dfVolumes = append(dfVolumes, &dfVolume{SystemDfVolumeReport: d})
	}
	volumeHeaders := "VOLUME NAME\tLINKS\tSIZE\n"
	volumeRow := "{{.VolumeName}}\t{{.Links}}\t{{.Size}}\n"
	format = volumeHeaders + "{{range . }}" + volumeRow + "{{end}}"
	return writeTemplate(w, format, dfVolumes)
}

func writeTemplate(w io.Writer, format string, output interface{}) error {
	tmpl, err := template.New("dfout").Parse(format)
	if err != nil {
		return err
	}
	w = tabwriter.NewWriter(w, 8, 2, 2, ' ', 0) //nolint
	if err := tmpl.Execute(w, output); err != nil {
		return err
	}
	if flusher, ok := w.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
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

func (d *dfContainer) Size() string {
	return units.HumanSize(float64(d.SystemDfContainerReport.Size))
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
