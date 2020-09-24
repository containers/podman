package image

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	sysreg "github.com/containers/podman/v2/pkg/registries"
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
	// Tag is the image tag
	Tag string
}

// SearchOptions are used to control the behaviour of SearchImages.
type SearchOptions struct {
	// Filter allows to filter the results.
	Filter SearchFilter
	// Limit limits the number of queries per index (default: 25). Must be
	// greater than 0 to overwrite the default value.
	Limit int
	// NoTrunc avoids the output to be truncated.
	NoTrunc bool
	// Authfile is the path to the authentication file.
	Authfile string
	// InsecureSkipTLSVerify allows to skip TLS verification.
	InsecureSkipTLSVerify types.OptionalBool
	// ListTags returns the search result with available tags
	ListTags bool
}

// SearchFilter allows filtering the results of SearchImages.
type SearchFilter struct {
	// Stars describes the minimal amount of starts of an image.
	Stars int
	// IsAutomated decides if only images from automated builds are displayed.
	IsAutomated types.OptionalBool
	// IsOfficial decides if only official images are displayed.
	IsOfficial types.OptionalBool
}

// SearchImages searches images based on term and the specified SearchOptions
// in all registries.
func SearchImages(term string, options SearchOptions) ([]SearchResult, error) {
	registry := ""

	// Try to extract a registry from the specified search term.  We
	// consider everything before the first slash to be the registry.  Note
	// that we cannot use the reference parser from the containers/image
	// library as the search term may container arbitrary input such as
	// wildcards.  See bugzilla.redhat.com/show_bug.cgi?id=1846629.
	if spl := strings.SplitN(term, "/", 2); len(spl) > 1 {
		registry = spl[0]
		term = spl[1]
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
		searchOutput := searchImageInRegistry(term, registry, options)
		data[index] = searchOutputData{data: searchOutput}
	}

	ctx := context.Background()
	for i := range registries {
		if err := sem.Acquire(ctx, 1); err != nil {
			return nil, err
		}
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

func searchImageInRegistry(term string, registry string, options SearchOptions) []SearchResult {
	// Max number of queries by default is 25
	limit := maxQueries
	if options.Limit > 0 {
		limit = options.Limit
	}

	sc := GetSystemContext("", options.Authfile, false)
	sc.DockerInsecureSkipTLSVerify = options.InsecureSkipTLSVerify
	// FIXME: Set this more globally.  Probably no reason not to have it in
	// every types.SystemContext, and to compute the value just once in one
	// place.
	sc.SystemRegistriesConfPath = sysreg.SystemRegistriesConfPath()
	if options.ListTags {
		results, err := searchRepositoryTags(registry, term, sc, options)
		if err != nil {
			logrus.Errorf("error listing registry tags %q: %v", registry, err)
			return []SearchResult{}
		}
		return results
	}

	results, err := docker.SearchRegistry(context.TODO(), sc, registry, term, limit)
	if err != nil {
		logrus.Errorf("error searching registry %q: %v", registry, err)
		return []SearchResult{}
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
	if options.Limit != 0 {
		limit = len(results)
		if options.Limit < len(results) {
			limit = options.Limit
		}
	}

	paramsArr := []SearchResult{}
	for i := 0; i < limit; i++ {
		// Check whether query matches filters
		if !(options.Filter.matchesAutomatedFilter(results[i]) && options.Filter.matchesOfficialFilter(results[i]) && options.Filter.matchesStarFilter(results[i])) {
			continue
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
	return paramsArr
}

func searchRepositoryTags(registry, term string, sc *types.SystemContext, options SearchOptions) ([]SearchResult, error) {
	dockerPrefix := fmt.Sprintf("%s://", docker.Transport.Name())
	imageRef, err := alltransports.ParseImageName(fmt.Sprintf("%s/%s", registry, term))
	if err == nil && imageRef.Transport().Name() != docker.Transport.Name() {
		return nil, errors.Errorf("reference %q must be a docker reference", term)
	} else if err != nil {
		imageRef, err = alltransports.ParseImageName(fmt.Sprintf("%s%s", dockerPrefix, fmt.Sprintf("%s/%s", registry, term)))
		if err != nil {
			return nil, errors.Errorf("reference %q must be a docker reference", term)
		}
	}
	tags, err := docker.GetRepositoryTags(context.TODO(), sc, imageRef)
	if err != nil {
		return nil, errors.Errorf("error getting repository tags: %v", err)
	}
	limit := maxQueries
	if len(tags) < limit {
		limit = len(tags)
	}
	if options.Limit != 0 {
		limit = len(tags)
		if options.Limit < limit {
			limit = options.Limit
		}
	}
	paramsArr := []SearchResult{}
	for i := 0; i < limit; i++ {
		params := SearchResult{
			Name: imageRef.DockerReference().Name(),
			Tag:  tags[i],
		}
		paramsArr = append(paramsArr, params)
	}
	return paramsArr, nil
}

// ParseSearchFilter turns the filter into a SearchFilter that can be used for
// searching images.
func ParseSearchFilter(filter []string) (*SearchFilter, error) {
	sFilter := new(SearchFilter)
	for _, f := range filter {
		arr := strings.SplitN(f, "=", 2)
		switch arr[0] {
		case "stars":
			if len(arr) < 2 {
				return nil, errors.Errorf("invalid `stars` filter %q, should be stars=<value>", filter)
			}
			stars, err := strconv.Atoi(arr[1])
			if err != nil {
				return nil, errors.Wrapf(err, "incorrect value type for stars filter")
			}
			sFilter.Stars = stars
		case "is-automated":
			if len(arr) == 2 && arr[1] == "false" {
				sFilter.IsAutomated = types.OptionalBoolFalse
			} else {
				sFilter.IsAutomated = types.OptionalBoolTrue
			}
		case "is-official":
			if len(arr) == 2 && arr[1] == "false" {
				sFilter.IsOfficial = types.OptionalBoolFalse
			} else {
				sFilter.IsOfficial = types.OptionalBoolTrue
			}
		default:
			return nil, errors.Errorf("invalid filter type %q", f)
		}
	}
	return sFilter, nil
}

func (f *SearchFilter) matchesStarFilter(result docker.SearchResult) bool {
	return result.StarCount >= f.Stars
}

func (f *SearchFilter) matchesAutomatedFilter(result docker.SearchResult) bool {
	if f.IsAutomated != types.OptionalBoolUndefined {
		return result.IsAutomated == (f.IsAutomated == types.OptionalBoolTrue)
	}
	return true
}

func (f *SearchFilter) matchesOfficialFilter(result docker.SearchResult) bool {
	if f.IsOfficial != types.OptionalBoolUndefined {
		return result.IsOfficial == (f.IsOfficial == types.OptionalBoolTrue)
	}
	return true
}
