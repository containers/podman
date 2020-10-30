package images

import (
	"os"
	"text/tabwriter"
	"text/template"

	"github.com/containers/common/pkg/auth"
	"github.com/containers/common/pkg/report"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v2/cmd/podman/parse"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// searchOptionsWrapper wraps entities.ImagePullOptions and prevents leaking
// CLI-only fields into the API types.
type searchOptionsWrapper struct {
	entities.ImageSearchOptions
	// CLI only flags
	TLSVerifyCLI bool   // Used to convert to an optional bool later
	Format       string // For go templating
}

var (
	searchOptions     = searchOptionsWrapper{}
	searchDescription = `Search registries for a given image. Can search all the default registries or a specific registry.

	Users can limit the number of results, and filter the output based on certain conditions.`

	// Command: podman search
	searchCmd = &cobra.Command{
		Use:   "search [options] TERM",
		Short: "Search registry for image",
		Long:  searchDescription,
		RunE:  imageSearch,
		Args:  cobra.ExactArgs(1),
		Example: `podman search --filter=is-official --limit 3 alpine
  podman search registry.fedoraproject.org/  # only works with v2 registries
  podman search --format "table {{.Index}} {{.Name}}" registry.fedoraproject.org/fedora`,
	}

	// Command: podman image search
	imageSearchCmd = &cobra.Command{
		Use:         searchCmd.Use,
		Short:       searchCmd.Short,
		Long:        searchCmd.Long,
		RunE:        searchCmd.RunE,
		Args:        searchCmd.Args,
		Annotations: searchCmd.Annotations,
		Example: `podman image search --filter=is-official --limit 3 alpine
  podman image search registry.fedoraproject.org/  # only works with v2 registries
  podman image search --format "table {{.Index}} {{.Name}}" registry.fedoraproject.org/fedora`,
	}
)

func init() {
	// search
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: searchCmd,
	})

	flags := searchCmd.Flags()
	searchFlags(flags)

	// images search
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: imageSearchCmd,
		Parent:  imageCmd,
	})

	imageSearchFlags := imageSearchCmd.Flags()
	searchFlags(imageSearchFlags)
}

// searchFlags set the flags for the pull command.
func searchFlags(flags *pflag.FlagSet) {
	flags.StringSliceVarP(&searchOptions.Filters, "filter", "f", []string{}, "Filter output based on conditions provided (default [])")
	flags.StringVar(&searchOptions.Format, "format", "", "Change the output format to a Go template")
	flags.IntVar(&searchOptions.Limit, "limit", 0, "Limit the number of results")
	flags.BoolVar(&searchOptions.NoTrunc, "no-trunc", false, "Do not truncate the output")
	flags.StringVar(&searchOptions.Authfile, "authfile", auth.GetDefaultAuthFile(), "Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
	flags.BoolVar(&searchOptions.TLSVerifyCLI, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries")
	flags.BoolVar(&searchOptions.ListTags, "list-tags", false, "List the tags of the input registry")
}

// imageSearch implements the command for searching images.
func imageSearch(cmd *cobra.Command, args []string) error {
	searchTerm := ""
	switch len(args) {
	case 1:
		searchTerm = args[0]
	default:
		return errors.Errorf("search requires exactly one argument")
	}

	if searchOptions.ListTags && len(searchOptions.Filters) != 0 {
		return errors.Errorf("filters are not applicable to list tags result")
	}

	// TLS verification in c/image is controlled via a `types.OptionalBool`
	// which allows for distinguishing among set-true, set-false, unspecified
	// which is important to implement a sane way of dealing with defaults of
	// boolean CLI flags.
	if cmd.Flags().Changed("tls-verify") {
		searchOptions.SkipTLSVerify = types.NewOptionalBool(!searchOptions.TLSVerifyCLI)
	}

	if searchOptions.Authfile != "" {
		if _, err := os.Stat(searchOptions.Authfile); err != nil {
			return err
		}
	}

	searchReport, err := registry.ImageEngine().Search(registry.GetContext(), searchTerm, searchOptions.ImageSearchOptions)
	if err != nil {
		return err
	}

	if len(searchReport) == 0 {
		return nil
	}

	hdrs := report.Headers(entities.ImageSearchReport{}, nil)
	renderHeaders := true
	var row string
	switch {
	case searchOptions.ListTags:
		if len(searchOptions.Filters) != 0 {
			return errors.Errorf("filters are not applicable to list tags result")
		}
		row = "{{.Name}}\t{{.Tag}}\n"
	case cmd.Flags().Changed("format"):
		renderHeaders = parse.HasTable(searchOptions.Format)
		row = report.NormalizeFormat(searchOptions.Format)
	default:
		row = "{{.Index}}\t{{.Name}}\t{{.Description}}\t{{.Stars}}\t{{.Official}}\t{{.Automated}}\n"
	}
	format := parse.EnforceRange(row)

	tmpl, err := template.New("search").Parse(format)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 8, 2, 2, ' ', 0)
	defer w.Flush()

	if renderHeaders {
		if err := tmpl.Execute(w, hdrs); err != nil {
			return errors.Wrapf(err, "failed to write search column headers")
		}
	}

	return tmpl.Execute(w, searchReport)
}
