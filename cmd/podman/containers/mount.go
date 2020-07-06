package containers

import (
	"fmt"
	"os"
	"text/tabwriter"
	"text/template"

	"github.com/containers/libpod/v2/cmd/podman/registry"
	"github.com/containers/libpod/v2/cmd/podman/utils"
	"github.com/containers/libpod/v2/cmd/podman/validate"
	"github.com/containers/libpod/v2/pkg/domain/entities"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	mountDescription = `podman mount
    Lists all mounted containers mount points if no container is specified

  podman mount CONTAINER-NAME-OR-ID
    Mounts the specified container and outputs the mountpoint
`

	mountCommand = &cobra.Command{
		Use:   "mount [flags] [CONTAINER]",
		Short: "Mount a working container's root filesystem",
		Long:  mountDescription,
		RunE:  mount,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndCIDFile(cmd, args, true, false)
		},
	}

	containerMountCommmand = &cobra.Command{
		Use:   mountCommand.Use,
		Short: mountCommand.Short,
		Long:  mountCommand.Long,
		RunE:  mountCommand.RunE,
	}
)

var (
	mountOpts entities.ContainerMountOptions
)

func mountFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&mountOpts.All, "all", "a", false, "Mount all containers")
	flags.StringVar(&mountOpts.Format, "format", "", "Change the output format to Go template")
	flags.BoolVar(&mountOpts.NoTruncate, "notruncate", false, "Do not truncate output")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: mountCommand,
	})
	mountFlags(mountCommand.Flags())
	validate.AddLatestFlag(mountCommand, &mountOpts.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: containerMountCommmand,
		Parent:  containerCmd,
	})
	mountFlags(containerMountCommmand.Flags())
	validate.AddLatestFlag(containerMountCommmand, &mountOpts.Latest)
}

func mount(_ *cobra.Command, args []string) error {
	var (
		errs utils.OutputErrors
	)
	reports, err := registry.ContainerEngine().ContainerMount(registry.GetContext(), args, mountOpts)
	if err != nil {
		return err
	}
	if len(args) > 0 || mountOpts.Latest || mountOpts.All {
		for _, r := range reports {
			if r.Err == nil {
				fmt.Println(r.Path)
				continue
			}
			errs = append(errs, r.Err)
		}
		return errs.PrintErrors()
	}
	if mountOpts.Format == "json" {
		return printJSON(reports)
	}
	mrs := make([]mountReporter, 0, len(reports))
	for _, r := range reports {
		mrs = append(mrs, mountReporter{r})
	}
	row := "{{.ID}} {{.Path}}"
	format := "{{range . }}" + row + "{{end}}"
	tmpl, err := template.New("mounts").Parse(format)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 8, 2, 2, ' ', 0)
	defer w.Flush()
	return tmpl.Execute(w, mrs)
}

func printJSON(reports []*entities.ContainerMountReport) error {
	type jreport struct {
		ID         string `json:"id"`
		Names      []string
		Mountpoint string `json:"mountpoint"`
	}
	jreports := make([]jreport, 0, len(reports))

	for _, r := range reports {
		jreports = append(jreports, jreport{
			ID:         r.Id,
			Names:      []string{r.Name},
			Mountpoint: r.Path,
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
	*entities.ContainerMountReport
}

func (m mountReporter) ID() string {
	if mountOpts.NoTruncate {
		return m.Id
	}
	return m.Id[0:12]
}
