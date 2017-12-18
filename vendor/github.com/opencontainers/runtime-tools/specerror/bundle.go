package specerror

import (
	"fmt"

	rfc2119 "github.com/opencontainers/runtime-tools/error"
)

// define error codes
const (
	// ConfigInRootBundleDir represents "This REQUIRED file MUST reside in the root of the bundle directory"
	ConfigInRootBundleDir = "This REQUIRED file MUST reside in the root of the bundle directory."
	// ConfigConstName represents "This REQUIRED file MUST be named `config.json`."
	ConfigConstName = "This REQUIRED file MUST be named `config.json`."
	// ArtifactsInSingleDir represents "When supplied, while these artifacts MUST all be present in a single directory on the local filesystem, that directory itself is not part of the bundle."
	ArtifactsInSingleDir = "When supplied, while these artifacts MUST all be present in a single directory on the local filesystem, that directory itself is not part of the bundle."
)

var (
	containerFormatRef = func(version string) (reference string, err error) {
		return fmt.Sprintf(referenceTemplate, version, "bundle.md#container-format"), nil
	}
)

func init() {
	register(ConfigInRootBundleDir, rfc2119.Must, containerFormatRef)
	register(ConfigConstName, rfc2119.Must, containerFormatRef)
	register(ArtifactsInSingleDir, rfc2119.Must, containerFormatRef)
}
