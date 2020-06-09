// Package bindings provides golang-based access
// to the Podman REST API.  Users can then interact with API endpoints
// to manage containers, images, pods, etc.
//
// This package exposes a series of methods that allow users to firstly
// create their connection with the API endpoints.  Once the connection
// is established, users can then manage the Podman container runtime.
package bindings

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/blang/semver"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	// PTrue is a convenience variable that can be used in bindings where
	// a pointer to a bool (optional parameter) is required.
	pTrue = true
	PTrue = &pTrue
	// PFalse is a convenience variable that can be used in bindings where
	// a pointer to a bool (optional parameter) is required.
	pFalse = false
	PFalse = &pFalse

	// APIVersion - podman will fail to run if this value is wrong
	APIVersion = semver.MustParse("1.0.0")
)

// readPassword prompts for a secret and returns value input by user from stdin
// Unlike terminal.ReadPassword(), $(echo $SECRET | podman...) is supported.
// Additionally, all input after `<secret>/n` is queued to podman command.
func readPassword(prompt string) (pw []byte, err error) {
	fd := int(os.Stdin.Fd())
	if terminal.IsTerminal(fd) {
		fmt.Fprint(os.Stderr, prompt)
		pw, err = terminal.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		return
	}

	var b [1]byte
	for {
		n, err := os.Stdin.Read(b[:])
		// terminal.ReadPassword discards any '\r', so we do the same
		if n > 0 && b[0] != '\r' {
			if b[0] == '\n' {
				return pw, nil
			}
			pw = append(pw, b[0])
			// limit size, so that a wrong input won't fill up the memory
			if len(pw) > 1024 {
				err = errors.New("password too long, 1024 byte limit")
			}
		}
		if err != nil {
			// terminal.ReadPassword accepts EOF-terminated passwords
			// if non-empty, so we do the same
			if err == io.EOF && len(pw) > 0 {
				err = nil
			}
			return pw, err
		}
	}
}
