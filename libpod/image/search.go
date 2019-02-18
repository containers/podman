package image

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/containers/image/docker"
	"github.com/containers/image/types"
	"github.com/containers/libpod/libpod/common"
	sysreg "github.com/containers/libpod/pkg/registries"
	"github.com/fatih/camelcase"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
)

const (
	descriptionTruncLength = 44
	maxQueries             = 25
	maxParallelSearches    = int64(6)
)

// SearchResult is holding image-search related data.
type SearchResult struct {
	// Index is the image index (e.g., "docker.io" or "quay.io")
	Index string
	// Name is the canoncical name of the image (e.g., "docker.io/library/alpine").
	Name string
	// Description of the image.
	Description string
	// Stars is the number of stars of the image.
	Stars int
	// Official indicates if it's an official image.
	Official string
	// Automated indicates if the image was created by an automated build.
	Automated string
}

// SearchOptions are used to control the behaviour of SearchImages.
type SearchOptions struct {
	// Filter allows to filter the results.
	Filter []string
	// Limit limits the number of queries per index (default: 25). Must be
	// greater than 0 to overwrite the default value.
	Limit int
	// NoTrunc avoids the output to be truncated.
	NoTrunc bool
	// Authfile is the path to the authentication file.
	Authfile string
	// InsecureSkipTLSVerify allows to skip TLS verification.
	InsecureSkipTLSVerify types.OptionalBool
}

type searchFilterParams struct {
	stars       int
	isAutomated *bool
	isOfficial  *bool
}

func splitCamelCase(src string) string {
	entries := camelcase.Split(src)
	return strings.Join(entries, " ")
}

// HeaderMap returns the headers of a SearchResult.
func (s *SearchResult) HeaderMap() map[string]string {
	v := reflect.Indirect(reflect.ValueOf(s))
	values := make(map[string]string, v.NumField())

	for i := 0; i < v.NumField(); i++ {
		key := v.Type().Field(i).Name
		value := key
		values[key] = strings.ToUpper(splitCamelCase(value))
	}
	return values
}

// SearchImages searches images based on term and the specified SearchOptions
// in all registries.
func SearchImages(term string, options SearchOptions) ([]SearchResult, error) {
	filter, err := parseSearchFilter(&options)
	if err != nil {
		return nil, err
	}

	// Check if search term has a registry in it
	registry, err := sysreg.GetRegistry(term)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting registry from %q", term)
	}
	if registry != "" {
		term = term[len(registry)+1:]
	}

	registries, err := getRegistries(registry)
	if err != nil {
		return nil, err
	}

	// searchOutputData is used as a return value for searching in parallel.
	type searchOutputData struct {
		data []SearchResult
		err  error
	}

	// Let's follow Firefox by limiting parallel downloads to 6.
	sem := semaphore.NewWeighted(maxParallelSearches)
	wg := sync.WaitGroup{}
	wg.Add(len(registries))
	data := make([]searchOutputData, len(registries))

	searchImageInRegistryHelper := func(index int, registry string) {
		defer sem.Release(1)
		defer wg.Done()
		searchOutput, err := searchImageInRegistry(term, registry, options, filter)
		data[index] = searchOutputData{data: searchOutput, err: err}
	}

	ctx := context.Background()
	for i := range registries {
		sem.Acquire(ctx, 1)
		go searchImageInRegistryHelper(i, registries[i])
	}

	wg.Wait()
	results := []SearchResult{}
	for _, d := range data {
		if d.err != nil {
			return nil, d.err
		}
		results = append(results, d.data...)
	}
	return results, nil
}

// getRegistries returns the list of registries to search, depending on an optional registry specification
func getRegistries(registry string) ([]string, error) {
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
	return registries, nil
}

func searchImageInRegistry(term string, registry string, options SearchOptions, filter *searchFilterParams) ([]SearchResult, error) {
	// Max number of queries by default is 25
	limit := maxQueries
	if options.Limit > 0 {
		limit = options.Limit
	}

	sc := common.GetSystemContext("", options.Authfile, false)
	sc.DockerInsecureSkipTLSVerify = options.InsecureSkipTLSVerify
	// FIXME: Set this more globally.  Probably no reason not to have it in
	// every types.SystemContext, and to compute the value just once in one
	// place.
	sc.SystemRegistriesConfPath = sysreg.SystemRegistriesConfPath()
	results, err := docker.SearchRegistry(context.TODO(), sc, registry, term, limit)
	if err != nil {
		logrus.Errorf("error searching registry %q: %v", registry, err)
		return []SearchResult{}, nil
	}
	index := registry
	arr := strings.Split(registry, ".")
	if len(arr) > 2 {
		index = strings.Join(arr[len(arr)-2:], ".")
	}

	// limit is the number of results to output
	// if the total number of results is less than the limit, output all
	// if the limit has been set by the user, output those number of queries
	limit = maxQueries
	if len(results) < limit {
		limit = len(results)
	}
	if options.Limit != 0 && options.Limit < len(results) {
		limit = options.Limit
	}

	paramsArr := []SearchResult{}
	for i := 0; i < limit; i++ {
		if len(options.Filter) > 0 {
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
		if len(description) > 44 && !options.NoTrunc {
			description = description[:descriptionTruncLength] + "..."
		}
		name := registry + "/" + results[i].Name
		if index == "docker.io" && !strings.Contains(results[i].Name, "/") {
			name = index + "/library/" + results[i].Name
		}
		params := SearchResult{
			Index:       index,
			Name:        name,
			Description: description,
			Official:    official,
			Automated:   automated,
			Stars:       results[i].StarCount,
		}
		paramsArr = append(paramsArr, params)
	}
	return paramsArr, nil
}

func parseSearchFilter(options *SearchOptions) (*searchFilterParams, error) {
	filterParams := &searchFilterParams{}
	ptrTrue := true
	ptrFalse := false
	for _, filter := range options.Filter {
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

func matchesStarFilter(filter *searchFilterParams, result docker.SearchResult) bool {
	return result.StarCount >= filter.stars
}

func matchesAutomatedFilter(filter *searchFilterParams, result docker.SearchResult) bool {
	if filter.isAutomated != nil {
		return result.IsAutomated == *filter.isAutomated
	}
	return true
}

func matchesOfficialFilter(filter *searchFilterParams, result docker.SearchResult) bool {
	if filter.isOfficial != nil {
		return result.IsOfficial == *filter.isOfficial
	}
	return true
}
