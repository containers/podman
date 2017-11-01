package alltransports

import (
	"strings"

	// register all known transports
	// NOTE: Make sure docs/policy.json.md is updated when adding or updating
	// a transport.
	_ "github.com/containers/image/directory"
	_ "github.com/containers/image/docker"
	_ "github.com/containers/image/docker/archive"
	_ "github.com/containers/image/docker/daemon"
	_ "github.com/containers/image/oci/archive"
	_ "github.com/containers/image/oci/layout"
	_ "github.com/containers/image/openshift"
	// The ostree transport is registered by ostree*.go
	_ "github.com/containers/image/storage"
	"github.com/containers/image/transports"
	"github.com/containers/image/types"
	"github.com/pkg/errors"
)

// ParseImageName converts a URL-like image name to a types.ImageReference.
func ParseImageName(imgName string) (types.ImageReference, error) {
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
