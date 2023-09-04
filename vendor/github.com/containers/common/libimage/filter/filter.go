package filter

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/containers/common/libimage/define"
	"github.com/containers/image/v5/types"
)

// SearchFilter allows filtering images while searching.
type SearchFilter struct {
	// Stars describes the minimal amount of starts of an image.
	Stars int
	// IsAutomated decides if only images from automated builds are displayed.
	IsAutomated types.OptionalBool
	// IsOfficial decides if only official images are displayed.
	IsOfficial types.OptionalBool
}

// ParseSearchFilter turns the filter into a SearchFilter that can be used for
// searching images.
func ParseSearchFilter(filter []string) (*SearchFilter, error) {
	sFilter := new(SearchFilter)
	for _, f := range filter {
		arr := strings.SplitN(f, "=", 2)
		switch arr[0] {
		case define.SearchFilterStars:
			if len(arr) < 2 {
				return nil, fmt.Errorf("invalid filter %q, should be stars=<value>", filter)
			}
			stars, err := strconv.Atoi(arr[1])
			if err != nil {
				return nil, fmt.Errorf("incorrect value type for stars filter: %w", err)
			}
			sFilter.Stars = stars
		case define.SearchFilterAutomated:
			if len(arr) == 2 && arr[1] == "false" {
				sFilter.IsAutomated = types.OptionalBoolFalse
			} else {
				sFilter.IsAutomated = types.OptionalBoolTrue
			}
		case define.SearchFilterOfficial:
			if len(arr) == 2 && arr[1] == "false" {
				sFilter.IsOfficial = types.OptionalBoolFalse
			} else {
				sFilter.IsOfficial = types.OptionalBoolTrue
			}
		default:
			return nil, fmt.Errorf("invalid filter type %q", f)
		}
	}
	return sFilter, nil
}
