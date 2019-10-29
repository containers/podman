package alltransports

import (
	"strings"

	// register all known transports
	// NOTE: Make sure docs/containers-policy.json.5.md is updated when adding or updating
	// a transport.
	_ "github.com/containers/image/v5/directory"
	_ "github.com/containers/image/v5/docker"
	_ "github.com/containers/image/v5/docker/archive"
	_ "github.com/containers/image/v5/oci/archive"
	_ "github.com/containers/image/v5/oci/layout"
	_ "github.com/containers/image/v5/openshift"
	_ "github.com/containers/image/v5/tarball"
	// The ostree transport is registered by ostree*.go
	// The storage transport is registered by storage*.go
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/types"
	"github.com/pkg/errors"
)

// ParseImageName converts a URL-like image name to a types.ImageReference.
func ParseImageName(imgName string) (types.ImageReference, error) {
	// Keep this in sync with TransportFromImageName!
	parts := strings.SplitN(imgName, ":", 2)
	if len(parts) != 2 {
		return nil, errors.Errorf(`Invalid image name "%s", expected colon-separated transport:reference`, imgName)
	}
	transport := transports.Get(parts[0])
	if transport == nil {
		return nil, errors.Errorf(`Invalid image name "%s", unknown transport "%s"`, imgName, parts[0])
	}
	return transport.ParseReference(parts[1])
}

// TransportFromImageName converts an URL-like name to a types.ImageTransport or nil when
// the transport is unknown or when the input is invalid.
func TransportFromImageName(imageName string) types.ImageTransport {
	// Keep this in sync with ParseImageName!
	parts := strings.SplitN(imageName, ":", 2)
	if len(parts) == 2 {
		return transports.Get(parts[0])
	}
	return nil
}
