// +build coverage

package main

import (
	"os"
	"strings"
	"testing"
)

// NOTE: do not use this in production.  Binaries built with this file are
// merely useful to collect coverage data.
func TestMain(_ *testing.T) {
	var (
		args []string
	)

	for _, arg := range os.Args {
		switch {
		case strings.HasPrefix(arg, "COVERAGE"):
			// Dummy argument to enable global flags for Podman.
		case strings.HasPrefix(arg, "-test"):
			// Make sure we don't pass `go test` specific flags to
			// Podman.
		default:
			args = append(args, arg)
		}
	}

	os.Args = args
	main() // "run" Podman

	// Make sure that std{err,out} write to /dev/null so we prevent the
	// testing backend to print "PASS" along with the coverage.  We really
	// want the coverage to be set via the `-test.coverprofile=$path` flag.
	null, _ := os.Open(os.DevNull)
	os.Stdout = null
	os.Stderr = null
}
