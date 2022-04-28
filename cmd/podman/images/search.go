package images

import (
	"fmt"
	"os"
	"strings"

	"github.com/containers/common/pkg/auth"
	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/report"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// searchOptionsWrapper wraps entities.ImagePullOptions and prevents leaking
// CLI-only fields into the API types.
type searchOptionsWrapper struct {
	entities.ImageSearchOptions
	// CLI only flags
	Compatible   bool   // Docker compat
	TLSVerifyCLI bool   // Used to convert to an optional bool later
	Format       string // For go templating
	NoTrunc      bool
}

// listEntryTag is a utility structure used for json serialization.
type listEntryTag struct {
	Name string
	Tags []string
}

var (
	searchOptions     = searchOptionsWrapper{}
	searchDescription = `Search registries for a given image. Can search all the default registries or a specific registry.

	Users can limit the number of results, and filter the output based on certain conditions.`

	searchCmd = &cobra.Command{
		Use:               "search [options] TERM",
		Short:             "Search registry for image",
		Long:              searchDescription,
		RunE:              imageSearch,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.AutocompleteNone,
		Example: `podman search --filter=is-official --limit 3 alpine
  podman search registry.fedoraproject.org/  # only works with v2 registries
  podman search --format "table {{.Index}} {{.Name}}" registry.fedoraproject.org/fedora`,
	}

	imageSearchCmd = &cobra.Command{
		Use:               searchCmd.Use,
		Short:             searchCmd.Short,
		Long:              searchCmd.Long,
		RunE:              searchCmd.RunE,
		Args:              searchCmd.Args,
		ValidArgsFunction: searchCmd.ValidArgsFunction,
		Example: `podman image search --filter=is-official --limit 3 alpine
  podman image search registry.fedoraproject.org/  # only works with v2 registries
  podman image search --format "table {{.Index}} {{.Name}}" registry.fedoraproject.org/fedora`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: searchCmd,
	})
	searchFlags(searchCmd)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: imageSearchCmd,
		Parent:  imageCmd,
	})
	searchFlags(imageSearchCmd)
}

// searchFlags set the flags for the pull command.
func searchFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	filterFlagName := "filter"
	flags.StringSliceVarP(&searchOptions.Filters, filterFlagName, "f", []string{}, "Filter output based on conditions provided (default [])")
	// TODO add custom filter function
	_ = cmd.RegisterFlagCompletionFunc(filterFlagName, completion.AutocompleteNone)

	formatFlagName := "format"
	flags.StringVar(&searchOptions.Format, formatFlagName, "", "Change the output format to JSON or a Go template")
	_ = cmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&entities.ImageSearchReport{}))

	limitFlagName := "limit"
	flags.IntVar(&searchOptions.Limit, limitFlagName, 0, "Limit the number of results")
	_ = cmd.RegisterFlagCompletionFunc(limitFlagName, completion.AutocompleteNone)

	flags.BoolVar(&searchOptions.NoTrunc, "no-trunc", false, "Do not truncate the output")
	flags.BoolVar(&searchOptions.Compatible, "compatible", false, "List stars, official and automated columns (Docker compatibility)")

	authfileFlagName := "authfile"
	flags.StringVar(&searchOptions.Authfile, authfileFlagName, auth.GetDefaultAuthFile(), "Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
	_ = cmd.RegisterFlagCompletionFunc(authfileFlagName, completion.AutocompleteDefault)

	flags.BoolVar(&searchOptions.TLSVerifyCLI, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries")
	flags.BoolVar(&searchOptions.ListTags, "list-tags", false, "List the tags of the input registry")
}

// imageSearch implements the command for searching images.
func imageSearch(cmd *cobra.Command, args []string) error {
	var searchTerm string
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

	isJSON := report.IsJSON(searchOptions.Format)
	for i, element := range searchReport {
		d := strings.ReplaceAll(element.Description, "\n", " ")
		if len(d) > 44 && !(searchOptions.NoTrunc || isJSON) {
			d = strings.TrimSpace(d[:44]) + "..."
		}
		searchReport[i].Description = d
	}

	rpt := report.New(os.Stdout, cmd.Name())
	defer rpt.Flush()

	switch {
	case searchOptions.ListTags:
		if len(searchOptions.Filters) != 0 {
			return errors.Errorf("filters are not applicable to list tags result")
		}
		if isJSON {
			listTagsEntries := buildListTagsJSON(searchReport)
			return printArbitraryJSON(listTagsEntries)
		}
		rpt, err = rpt.Parse(report.OriginPodman, "{{range .}}{{.Name}}\t{{.Tag}}\n{{end -}}")
	case isJSON:
		return printArbitraryJSON(searchReport)
	case cmd.Flags().Changed("format"):
		rpt, err = rpt.Parse(report.OriginUser, searchOptions.Format)
	default:
		row := "{{.Name}}\t{{.Description}}"
		if searchOptions.Compatible {
			row += "\t{{.Stars}}\t{{.Official}}\t{{.Automated}}"
		}
		row = "{{range . }}" + row + "\n{{end -}}"
		rpt, err = rpt.Parse(report.OriginPodman, row)
	}
	if err != nil {
		return err
	}

	if rpt.RenderHeaders {
		hdrs := report.Headers(entities.ImageSearchReport{}, nil)
		if err := rpt.Execute(hdrs); err != nil {
			return errors.Wrapf(err, "failed to write report column headers")
		}
	}
	return rpt.Execute(searchReport)
}

func printArbitraryJSON(v interface{}) error {
	prettyJSON, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		return err
	}
	fmt.Println(string(prettyJSON))
	return nil
}

func buildListTagsJSON(searchReport []entities.ImageSearchReport) []listEntryTag {
	entries := make([]listEntryTag, 0)

ReportLoop:
	for _, report := range searchReport {
		for idx, entry := range entries {
			if entry.Name == report.Name {
				entries[idx].Tags = append(entries[idx].Tags, report.Tag)
				continue ReportLoop
			}
		}
		newElem := listEntryTag{
			report.Name,
			[]string{report.Tag},
		}

		entries = append(entries, newElem)
	}
	return entries
}
