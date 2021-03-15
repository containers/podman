// Package fedoracoreos contains APIs defining well-known
// streams for Fedora CoreOS and a method to retrieve
// the URL for a stream endpoint.
package fedoracoreos

import (
	"fmt"
	"net/url"

	"github.com/coreos/stream-metadata-go/fedoracoreos/internals"
)

const (
	// StreamStable is the default stream
	StreamStable = "stable"
	// StreamTesting is what is intended to land in stable
	StreamTesting = "testing"
	// StreamNext usually tracks the next Fedora major version
	StreamNext = "next"
)

// GetStreamURL returns the URL for the given stream
func GetStreamURL(stream string) url.URL {
	u := internals.GetBaseURL()
	u.Path = fmt.Sprintf("streams/%s.json", stream)
	return u
}
