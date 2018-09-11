package main

import (
	"context"
	"reflect"
	"strconv"
	"strings"

	"github.com/containers/image/docker"
	"github.com/containers/libpod/cmd/podman/formats"
	"github.com/containers/libpod/libpod/common"
	sysreg "github.com/containers/libpod/pkg/registries"
	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	descriptionTruncLength = 44
	maxQueries             = 25
)

var (
	searchFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "authfile",
			Usage: "Path of the authentication file. Default is ${XDG_RUNTIME_DIR}/containers/auth.json",
		},
		cli.StringSliceFlag{
			Name:  "filter, f",
			Usage: "filter output based on conditions provided (default [])",
		},
		cli.StringFlag{
			Name:  "format",
			Usage: "change the output format to a Go template",
		},
		cli.IntFlag{
			Name:  "limit",
			Usage: "limit the number of results",
		},
		cli.BoolFlag{
			Name:  "no-trunc",
			Usage: "do not truncate the output",
		},
		cli.BoolTFlag{
			Name:  "tls-verify",
			Usage: "require HTTPS and verify certificates when contacting registries (default: true)",
		},
	}
	searchDescription = `
	Search registries for a given image. Can search all the default registries or a specific registry.
	Can limit the number of results, and filter the output based on certain conditions.`
	searchCommand = cli.Command{
		Name:         "search",
		Usage:        "Search registry for image",
		Description:  searchDescription,
		Flags:        searchFlags,
		Action:       searchCmd,
		ArgsUsage:    "TERM",
		OnUsageError: usageErrorHandler,
	}
)

type searchParams struct {
	Index       string
	Name        string
	Description string
	Stars       int
	Official    string
	Automated   string
}

type searchOpts struct {
	filter   []string
	limit    int
	noTrunc  bool
	format   string
	authfile string
}

type searchFilterParams struct {
	stars       int
	isAutomated *bool
	isOfficial  *bool
}

func searchCmd(c *cli.Context) error {
	args := c.Args()
	if len(args) > 1 {
		return errors.Errorf("too many arguments. Requires exactly 1")
	}
	if len(args) == 0 {
		return errors.Errorf("no argument given, requires exactly 1 argument")
	}
	term := args[0]

	// Check if search term has a registry in it
	registry, err := getRegistry(term)
	if err != nil {
		return errors.Wrapf(err, "error getting registry from %q", term)
	}
	if registry != "" {
		term = term[len(registry)+1:]
	}

	if err := validateFlags(c, searchFlags); err != nil {
		return err
	}

	format := genSearchFormat(c.String("format"))
	opts := searchOpts{
		format:   format,
		noTrunc:  c.Bool("no-trunc"),
		limit:    c.Int("limit"),
		filter:   c.StringSlice("filter"),
		authfile: c.String("authfile"),
	}
	regAndSkipTLS, err := getRegistriesAndSkipTLS(c, registry)
	if err != nil {
		return err
	}

	filter, err := parseSearchFilter(&opts)
	if err != nil {
		return err
	}

	return generateSearchOutput(term, regAndSkipTLS, opts, *filter)
}

func genSearchFormat(format string) string {
	if format != "" {
		// "\t" from the command line is not being recognized as a tab
		// replacing the string "\t" to a tab character if the user passes in "\t"
		return strings.Replace(format, `\t`, "\t", -1)
	}
	return "table {{.Index}}\t{{.Name}}\t{{.Description}}\t{{.Stars}}\t{{.Official}}\t{{.Automated}}\t"
}

func searchToGeneric(params []searchParams) (genericParams []interface{}) {
	for _, v := range params {
		genericParams = append(genericParams, interface{}(v))
	}
	return genericParams
}

func (s *searchParams) headerMap() map[string]string {
	v := reflect.Indirect(reflect.ValueOf(s))
	values := make(map[string]string, v.NumField())

	for i := 0; i < v.NumField(); i++ {
		key := v.Type().Field(i).Name
		value := key
		values[key] = strings.ToUpper(splitCamelCase(value))
	}
	return values
}

// A function for finding which registries can skip TLS
func getRegistriesAndSkipTLS(c *cli.Context, registry string) (map[string]bool, error) {
	// Variables for setting up Registry and TLSVerify
	tlsVerify := c.BoolT("tls-verify")
	forceSecure := false

	if c.IsSet("tls-verify") {
		forceSecure = c.BoolT("tls-verify")
	}

	var registries []string
	if registry != "" {
		registries = append(registries, registry)
	} else {
		var err error
		registries, err = sysreg.GetRegistries()
		if err != nil {
			return nil, errors.Wrapf(err, "error getting registries to search")
		}
	}
	regAndSkipTLS := make(map[string]bool)
	// If tls-verify is set to false, allow insecure always.
	if !tlsVerify {
		for _, reg := range registries {
			regAndSkipTLS[reg] = true
		}
	} else {
		// initially set all registries to verify with TLS
		for _, reg := range registries {
			regAndSkipTLS[reg] = false
		}
		// if the user didn't allow nor disallow insecure registries, check to see if the registry is insecure
		if !forceSecure {
			insecureRegistries, err := sysreg.GetInsecureRegistries()
			if err != nil {
				return nil, errors.Wrapf(err, "error getting insecure registries to search")
			}
			for _, reg := range insecureRegistries {
				// if there are any insecure registries in registries, allow for HTTP
				if _, ok := regAndSkipTLS[reg]; ok {
					regAndSkipTLS[reg] = true
				}
			}
		}
	}
	return regAndSkipTLS, nil
}

func getSearchOutput(term string, regAndSkipTLS map[string]bool, opts searchOpts, filter searchFilterParams) ([]searchParams, error) {
	// Max number of queries by default is 25
	limit := maxQueries
	if opts.limit != 0 {
		limit = opts.limit
	}

	sc := common.GetSystemContext("", opts.authfile, false)
	var paramsArr []searchParams
	for reg, skipTLS := range regAndSkipTLS {
		// set the SkipTLSVerify bool depending on the registry being searched through
		sc.DockerInsecureSkipTLSVerify = skipTLS
		results, err := docker.SearchRegistry(context.TODO(), sc, reg, term, limit)
		if err != nil {
			logrus.Errorf("error searching registry %q: %v", reg, err)
			continue
		}
		index := reg
		arr := strings.Split(reg, ".")
		if len(arr) > 2 {
			index = strings.Join(arr[len(arr)-2:], ".")
		}

		// limit is the number of results to output
		// if the total number of results is less than the limit, output all
		// if the limit has been set by the user, output those number of queries
		limit := maxQueries
		if len(results) < limit {
			limit = len(results)
		}
		if opts.limit != 0 && opts.limit < len(results) {
			limit = opts.limit
		}

		for i := 0; i < limit; i++ {
			if len(opts.filter) > 0 {
				// Check whether query matches filters
				if !(matchesAutomatedFilter(filter, results[i]) && matchesOfficialFilter(filter, results[i]) && matchesStarFilter(filter, results[i])) {
					continue
				}
			}
			official := ""
			if results[i].IsOfficial {
				official = "[OK]"
			}
			automated := ""
			if results[i].IsAutomated {
				automated = "[OK]"
			}
			description := strings.Replace(results[i].Description, "\n", " ", -1)
			if len(description) > 44 && !opts.noTrunc {
				description = description[:descriptionTruncLength] + "..."
			}
			name := reg + "/" + results[i].Name
			if index == "docker.io" && !strings.Contains(results[i].Name, "/") {
				name = index + "/library/" + results[i].Name
			}
			params := searchParams{
				Index:       index,
				Name:        name,
				Description: description,
				Official:    official,
				Automated:   automated,
				Stars:       results[i].StarCount,
			}
			paramsArr = append(paramsArr, params)
		}
	}
	return paramsArr, nil
}

func generateSearchOutput(term string, regAndSkipTLS map[string]bool, opts searchOpts, filter searchFilterParams) error {
	searchOutput, err := getSearchOutput(term, regAndSkipTLS, opts, filter)
	if err != nil {
		return err
	}
	if len(searchOutput) == 0 {
		return nil
	}
	out := formats.StdoutTemplateArray{Output: searchToGeneric(searchOutput), Template: opts.format, Fields: searchOutput[0].headerMap()}
	return formats.Writer(out).Out()
}

func parseSearchFilter(opts *searchOpts) (*searchFilterParams, error) {
	filterParams := &searchFilterParams{}
	ptrTrue := true
	ptrFalse := false
	for _, filter := range opts.filter {
		arr := strings.Split(filter, "=")
		switch arr[0] {
		case "stars":
			if len(arr) < 2 {
				return nil, errors.Errorf("invalid `stars` filter %q, should be stars=<value>", filter)
			}
			stars, err := strconv.Atoi(arr[1])
			if err != nil {
				return nil, errors.Wrapf(err, "incorrect value type for stars filter")
			}
			filterParams.stars = stars
			break
		case "is-automated":
			if len(arr) == 2 && arr[1] == "false" {
				filterParams.isAutomated = &ptrFalse
			} else {
				filterParams.isAutomated = &ptrTrue
			}
			break
		case "is-official":
			if len(arr) == 2 && arr[1] == "false" {
				filterParams.isOfficial = &ptrFalse
			} else {
				filterParams.isOfficial = &ptrTrue
			}
			break
		default:
			return nil, errors.Errorf("invalid filter type %q", filter)
		}
	}
	return filterParams, nil
}

func matchesStarFilter(filter searchFilterParams, result docker.SearchResult) bool {
	return result.StarCount >= filter.stars
}

func matchesAutomatedFilter(filter searchFilterParams, result docker.SearchResult) bool {
	if filter.isAutomated != nil {
		return result.IsAutomated == *filter.isAutomated
	}
	return true
}

func matchesOfficialFilter(filter searchFilterParams, result docker.SearchResult) bool {
	if filter.isOfficial != nil {
		return result.IsOfficial == *filter.isOfficial
	}
	return true
}

func getRegistry(image string) (string, error) {
	// It is possible to only have the registry name in the format "myregistry/"
	// if so, just trim the "/" from the end and return the registry name
	if strings.HasSuffix(image, "/") {
		return strings.TrimSuffix(image, "/"), nil
	}
	imgRef, err := reference.Parse(image)
	if err != nil {
		return "", err
	}
	return reference.Domain(imgRef.(reference.Named)), nil
}
