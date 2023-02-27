// Package fedoracoreos contains APIs defining well-known
// streams for Fedora CoreOS and a method to retrieve
// the URL for a stream endpoint.
package fedoracoreos

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/coreos/stream-metadata-go/fedoracoreos/internals"
	"github.com/coreos/stream-metadata-go/stream"
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

func getStream(u url.URL) (*stream.Stream, error) {
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}

	var s stream.Stream
	err = json.Unmarshal(body, &s)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// FetchStream fetches and parses stream metadata for a stream
func FetchStream(streamName string) (*stream.Stream, error) {
	u := GetStreamURL(streamName)
	s, err := getStream(u)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch stream %s: %w", streamName, err)
	}
	return s, nil
}
