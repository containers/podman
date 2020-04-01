package images

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/containers/buildah/pkg/formats"
	"github.com/containers/libpod/cmd/podmanV2/common"
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	// Command: podman image _inspect_
	inspectCmd = &cobra.Command{
		Use:     "inspect [flags] IMAGE",
		Short:   "Display the configuration of an image",
		Long:    `Displays the low-level information on an image identified by name or ID.`,
		RunE:    inspect,
		Example: `podman image inspect alpine`,
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
	latestContainer := inspectOpts.Latest

	if len(args) == 0 && !latestContainer {
		return errors.Errorf("container or image name must be specified: podman inspect [options [...]] name")
	}

	if len(args) > 0 && latestContainer {
		return errors.Errorf("you cannot provide additional arguments with --latest")
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
		err = tmpl.Execute(w, results)
		if err != nil {
			return err
		}
	}

	for id, e := range results.Errors {
		fmt.Fprintf(os.Stderr, "%s: %s\n", id, e.Error())
	}
	return nil
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
