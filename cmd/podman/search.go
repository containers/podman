package main

import (
	"os"
	"reflect"
	"strings"

	buildahcli "github.com/containers/buildah/pkg/cli"
	"github.com/containers/buildah/pkg/formats"
	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod/image"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	searchCommand     cliconfig.SearchValues
	searchDescription = `Search registries for a given image. Can search all the default registries or a specific registry.

  Users can limit the number of results, and filter the output based on certain conditions.`
	_searchCommand = &cobra.Command{
		Use:   "search [flags] TERM",
		Short: "Search registry for image",
		Long:  searchDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			searchCommand.InputArgs = args
			searchCommand.GlobalFlags = MainGlobalOpts
			searchCommand.Remote = remoteclient
			return searchCmd(&searchCommand)
		},
		Example: `podman search --filter=is-official --limit 3 alpine
  podman search registry.fedoraproject.org/  # only works with v2 registries
  podman search --format "table {{.Index}} {{.Name}}" registry.fedoraproject.org/fedora`,
	}
)

func init() {
	searchCommand.Command = _searchCommand
	searchCommand.SetHelpTemplate(HelpTemplate())
	searchCommand.SetUsageTemplate(UsageTemplate())
	flags := searchCommand.Flags()
	flags.StringSliceVarP(&searchCommand.Filter, "filter", "f", []string{}, "Filter output based on conditions provided (default [])")
	flags.StringVar(&searchCommand.Format, "format", "", "Change the output format to a Go template")
	flags.IntVar(&searchCommand.Limit, "limit", 0, "Limit the number of results")
	flags.BoolVar(&searchCommand.NoTrunc, "no-trunc", false, "Do not truncate the output")
	// Disabled flags for the remote client
	if !remote {
		flags.StringVar(&searchCommand.Authfile, "authfile", buildahcli.GetDefaultAuthFile(), "Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
		flags.BoolVar(&searchCommand.TlsVerify, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries")
	}
}

func searchCmd(c *cliconfig.SearchValues) error {
	args := c.InputArgs
	if len(args) > 1 {
		return errors.Errorf("too many arguments. Requires exactly 1")
	}
	if len(args) == 0 {
		return errors.Errorf("no argument given, requires exactly 1 argument")
	}
	term := args[0]

	filter, err := image.ParseSearchFilter(c.Filter)
	if err != nil {
		return err
	}

	if c.Authfile != "" {
		if _, err := os.Stat(c.Authfile); err != nil {
			return errors.Wrapf(err, "error getting authfile %s", c.Authfile)
		}
	}

	searchOptions := image.SearchOptions{
		NoTrunc:  c.NoTrunc,
		Limit:    c.Limit,
		Filter:   *filter,
		Authfile: c.Authfile,
	}
	if c.Flag("tls-verify").Changed {
		searchOptions.InsecureSkipTLSVerify = types.NewOptionalBool(!c.TlsVerify)
	}

	results, err := image.SearchImages(term, searchOptions)
	if err != nil {
		return err
	}
	format := genSearchFormat(c.Format)
	if len(results) == 0 {
		return nil
	}
	out := formats.StdoutTemplateArray{Output: searchToGeneric(results), Template: format, Fields: searchHeaderMap()}
	return out.Out()
}

// searchHeaderMap returns the headers of a SearchResult.
func searchHeaderMap() map[string]string {
	s := new(image.SearchResult)
	v := reflect.Indirect(reflect.ValueOf(s))
	values := make(map[string]string, v.NumField())

	for i := 0; i < v.NumField(); i++ {
		key := v.Type().Field(i).Name
		value := key
		values[key] = strings.ToUpper(splitCamelCase(value))
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

func searchToGeneric(params []image.SearchResult) (genericParams []interface{}) {
	for _, v := range params {
		genericParams = append(genericParams, interface{}(v))
	}
	return genericParams
}
