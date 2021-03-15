// Package internals contains functions for accessing
// the underlying "releases" and coreos-assembler builds
// backing streams.  General user code should avoid
// this package and use streams.
package internals

import (
	"fmt"
	"net/url"
)

// GetBaseURL returns the base URL
func GetBaseURL() url.URL {
	return url.URL{
		Scheme: "https",
		Host:   "builds.coreos.fedoraproject.org",
	}
}

// GetReleaseIndexURL returns the URL for the release index of a given stream.
// Avoid this unless you have a specific need to test a specific release.
func GetReleaseIndexURL(stream string) url.URL {
	u := GetBaseURL()
	u.Path = fmt.Sprintf("prod/streams/%s/releases.json", stream)
	return u
}

// GetCosaBuild returns the coreos-assembler build URL
func GetCosaBuild(stream, buildID, arch string) url.URL {
	u := GetBaseURL()
	u.Path = fmt.Sprintf("prod/streams/%s/builds/%s/%s/", stream, buildID, arch)
	return u
}
