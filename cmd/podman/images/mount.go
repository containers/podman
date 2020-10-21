package images

import (
	"fmt"
	"os"
	"text/tabwriter"
	"text/template"

	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/utils"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	mountDescription = `podman image mount
    Lists all mounted images mount points if no images is specified

  podman image mount IMAGE-NAME-OR-ID
    Mounts the specified image and prints the mountpoint
`

	mountCommand = &cobra.Command{
		Use:   "mount [options] [IMAGE...]",
		Short: "Mount an image's root filesystem",
		Long:  mountDescription,
		RunE:  mount,
		Example: `podman image mount imgID
  podman image mount imgID1 imgID2 imgID3
  podman image mount
  podman image mount --all`,
		Annotations: map[string]string{
			registry.UnshareNSRequired: "",
			registry.ParentNSRequired:  "",
		},
	}
)

var (
	mountOpts entities.ImageMountOptions
)

func mountFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&mountOpts.All, "all", "a", false, "Mount all images")
	flags.StringVar(&mountOpts.Format, "format", "", "Print the mounted images in specified format (json)")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: mountCommand,
		Parent:  imageCmd,
	})
	mountFlags(mountCommand.Flags())
}

func mount(cmd *cobra.Command, args []string) error {
	if len(args) > 0 && mountOpts.All {
		return errors.New("when using the --all switch, you may not pass any image names or IDs")
	}

	reports, err := registry.ImageEngine().Mount(registry.GetContext(), args, mountOpts)
	if err != nil {
		return err
	}

	if len(args) > 0 || mountOpts.All {
		var errs utils.OutputErrors
		for _, r := range reports {
			if r.Err == nil {
				fmt.Println(r.Path)
				continue
			}
			errs = append(errs, r.Err)
		}
		return errs.PrintErrors()
	}

	switch {
	case report.IsJSON(mountOpts.Format):
		return printJSON(reports)
	case mountOpts.Format == "":
		break // default format
	default:
		return errors.Errorf("unknown --format argument: %q", mountOpts.Format)
	}

	mrs := make([]mountReporter, 0, len(reports))
	for _, r := range reports {
		mrs = append(mrs, mountReporter{r})
	}

	row := "{{range . }}{{.ID}}\t{{.Path}}\n{{end}}"
	tmpl, err := template.New("mounts").Parse(row)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 8, 2, 2, ' ', 0)
	defer w.Flush()
	return tmpl.Execute(w, mrs)
}

func printJSON(reports []*entities.ImageMountReport) error {
	type jreport struct {
		ID           string `json:"id"`
		Names        []string
		Repositories []string
		Mountpoint   string `json:"mountpoint"`
	}
	jreports := make([]jreport, 0, len(reports))

	for _, r := range reports {
		jreports = append(jreports, jreport{
			ID:           r.Id,
			Names:        []string{r.Name},
			Repositories: r.Repositories,
			Mountpoint:   r.Path,
		})
	}
	b, err := json.MarshalIndent(jreports, "", " ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

type mountReporter struct {
	*entities.ImageMountReport
}

func (m mountReporter) ID() string {
	if len(m.Repositories) > 0 {
		return m.Repositories[0]
	}
	return m.Id
}
