package copy

import (
	"strings"

	"github.com/pkg/errors"
)

// ParseSourceAndDestination parses the source and destination input into a
// possibly specified container and path.  The input format is described in
// podman-cp(1) as "[nameOrID:]path".  Colons in paths are supported as long
// they start with a dot or slash.
//
// It returns, in order, the source container and path, followed by the
// destination container and path, and an error.  Note that exactly one
// container must be specified.
func ParseSourceAndDestination(source, destination string) (string, string, string, string, error) {
	sourceContainer, sourcePath := parseUserInput(source)
	destContainer, destPath := parseUserInput(destination)

	numContainers := 0
	if len(sourceContainer) > 0 {
		numContainers++
	}
	if len(destContainer) > 0 {
		numContainers++
	}

	if numContainers != 1 {
		return "", "", "", "", errors.Errorf("invalid arguments %q, %q: exactly 1 container expected but %d specified", source, destination, numContainers)
	}

	if len(sourcePath) == 0 || len(destPath) == 0 {
		return "", "", "", "", errors.Errorf("invalid arguments %q, %q: you must specify paths", source, destination)
	}

	return sourceContainer, sourcePath, destContainer, destPath, nil
}

// parseUserInput parses the input string and returns, if specified, the name
// or ID of the container and the path.  The input format is described in
// podman-cp(1) as "[nameOrID:]path".  Colons in paths are supported as long
// they start with a dot or slash.
func parseUserInput(input string) (container string, path string) {
	if len(input) == 0 {
		return
	}
	path = input

	// If the input starts with a dot or slash, it cannot refer to a
	// container.
	if input[0] == '.' || input[0] == '/' {
		return
	}

	if spl := strings.SplitN(path, ":", 2); len(spl) == 2 {
		container = spl[0]
		path = spl[1]
	}
	return
}
