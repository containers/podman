package images

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/containers/buildah/pkg/formats"
	"github.com/containers/libpod/cmd/podman/common"
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	// Command: podman image _inspect_
	inspectCmd = &cobra.Command{
		Use:   "inspect [flags] IMAGE",
		Short: "Display the configuration of an image",
		Long:  `Displays the low-level information on an image identified by name or ID.`,
		RunE:  inspect,
		Example: `podman inspect alpine
  podman inspect --format "imageId: {{.Id}} size: {{.Size}}" alpine
  podman inspect --format "image: {{.ImageName}} driver: {{.Driver}}" myctr`,
	}
	inspectOpts *entities.InspectOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: inspectCmd,
		Parent:  imageCmd,
	})
	inspectOpts = common.AddInspectFlagSet(inspectCmd)
}

func inspect(cmd *cobra.Command, args []string) error {
	if inspectOpts.Size {
		return fmt.Errorf("--size can only be used for containers")
	}
	if inspectOpts.Latest {
		return fmt.Errorf("--latest can only be used for containers")
	}
	if len(args) == 0 {
		return errors.Errorf("image name must be specified: podman image inspect [options [...]] name")
	}

	results, err := registry.ImageEngine().Inspect(context.Background(), args, *inspectOpts)
	if err != nil {
		return err
	}

	if len(results.Images) > 0 {
		if inspectOpts.Format == "" {
			buf, err := json.MarshalIndent(results.Images, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(buf))

			for id, e := range results.Errors {
				fmt.Fprintf(os.Stderr, "%s: %s\n", id, e.Error())
			}
			return nil
		}
		row := inspectFormat(inspectOpts.Format)
		format := "{{range . }}" + row + "{{end}}"
		tmpl, err := template.New("inspect").Parse(format)
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 8, 2, 2, ' ', 0)
		defer func() { _ = w.Flush() }()
		err = tmpl.Execute(w, results.Images)
		if err != nil {
			return err
		}
	}

	var lastErr error
	for id, e := range results.Errors {
		if lastErr != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", id, lastErr.Error())
		}
		lastErr = e
	}
	return lastErr
}

func inspectFormat(row string) string {
	r := strings.NewReplacer("{{.Id}}", formats.IDString,
		".Src", ".Source",
		".Dst", ".Destination",
		".ImageID", ".Image",
	)
	row = r.Replace(row)

	if !strings.HasSuffix(row, "\n") {
		row += "\n"
	}
	return row
}

func Inspect(cmd *cobra.Command, args []string, options *entities.InspectOptions) error {
	inspectOpts = options
	return inspect(cmd, args)
}
