package alltransports

import (
	"fmt"
	"strings"

	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/types"

	// Register all known transports.
	// NOTE: Make sure docs/containers-transports.5.md and docs/containers-policy.json.5.md are updated when adding or updating
	// a transport.
	_ "github.com/containers/image/v5/directory"
	_ "github.com/containers/image/v5/docker"
	_ "github.com/containers/image/v5/docker/archive"
	_ "github.com/containers/image/v5/oci/archive"
	_ "github.com/containers/image/v5/oci/layout"
	_ "github.com/containers/image/v5/openshift"
	_ "github.com/containers/image/v5/sif"
	_ "github.com/containers/image/v5/tarball"
	// The docker-daemon transport is registeredy by docker_daemon*.go
	// The ostree transport is registered by ostree*.go
	// The storage transport is registered by storage*.go
)

// ParseImageName converts a URL-like image name to a types.ImageReference.
func ParseImageName(imgName string) (types.ImageReference, error) {
	// Keep this in sync with TransportFromImageName!
	transportName, withinTransport, valid := strings.Cut(imgName, ":")
	if !valid {
		return nil, fmt.Errorf(`Invalid image name %q, expected colon-separated transport:reference`, imgName)
	}
	transport := transports.Get(transportName)
	if transport == nil {
		return nil, fmt.Errorf(`Invalid image name %q, unknown transport %q`, imgName, transportName)
	}
	return transport.ParseReference(withinTransport)
}

// TransportFromImageName converts an URL-like name to a types.ImageTransport or nil when
// the transport is unknown or when the input is invalid.
func TransportFromImageName(imageName string) types.ImageTransport {
	// Keep this in sync with ParseImageName!
	transportName, _, valid := strings.Cut(imageName, ":")
	if valid {
		return transports.Get(transportName)
	}
	return nil
}
