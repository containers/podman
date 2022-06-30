package parse

import (
	"fmt"
	"net/url"
	"strings"
)

func FilterArgumentsIntoFilters(filters []string) (url.Values, error) {
	parsedFilters := make(url.Values)
	for _, f := range filters {
		t := strings.SplitN(f, "=", 2)
		if len(t) < 2 {
			return parsedFilters, fmt.Errorf("filter input must be in the form of filter=value: %s is invalid", f)
		}
		parsedFilters.Add(t[0], t[1])
	}
	return parsedFilters, nil
}
