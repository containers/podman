package images

import (
	"os"
	"reflect"
	"strings"

	"github.com/containers/buildah/pkg/formats"
	"github.com/containers/common/pkg/auth"
	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/util/camelcase"
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
		Use:   "search [flags] TERM",
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
	if registry.IsRemote() {
		_ = flags.MarkHidden("authfile")
		_ = flags.MarkHidden("tls-verify")
	}
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

	if searchOptions.Limit > 100 {
		return errors.Errorf("Limit %d is outside the range of [1, 100]", searchOptions.Limit)
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
			return errors.Wrapf(err, "error getting authfile %s", searchOptions.Authfile)
		}
	}

	searchReport, err := registry.ImageEngine().Search(registry.GetContext(), searchTerm, searchOptions.ImageSearchOptions)
	if err != nil {
		return err
	}

	format := genSearchFormat(searchOptions.Format)
	if len(searchReport) == 0 {
		return nil
	}
	out := formats.StdoutTemplateArray{Output: searchToGeneric(searchReport), Template: format, Fields: searchHeaderMap()}
	return out.Out()
}

// searchHeaderMap returns the headers of a SearchResult.
func searchHeaderMap() map[string]string {
	s := new(entities.ImageSearchReport)
	v := reflect.Indirect(reflect.ValueOf(s))
	values := make(map[string]string, v.NumField())

	for i := 0; i < v.NumField(); i++ {
		key := v.Type().Field(i).Name
		value := key
		values[key] = strings.ToUpper(strings.Join(camelcase.Split(value), " "))
	}
	return values
}

func genSearchFormat(format string) string {
	if format != "" {
		// "\t" from the command line is not being recognized as a tab
		// replacing the string "\t" to a tab character if the user passes in "\t"
		return strings.Replace(format, `\t`, "\t", -1)
	}
	return "table {{.Index}}\t{{.Name}}\t{{.Description}}\t{{.Stars}}\t{{.Official}}\t{{.Automated}}\t"
}

func searchToGeneric(params []entities.ImageSearchReport) (genericParams []interface{}) {
	for _, v := range params {
		genericParams = append(genericParams, interface{}(v))
	}
	return genericParams
}
