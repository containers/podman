package common

import (
	"os"

	"github.com/containers/image/v5/types"
)

// SetRegistriesConfPath sets the registries.conf path for the specified context.
// NOTE: this is a verbatim copy from c/common/libimage which we're not using
// to prevent leaking c/storage into this file.  Maybe this should go into c/image?
func SetRegistriesConfPath(systemContext *types.SystemContext) {
	if systemContext.SystemRegistriesConfPath != "" {
		return
	}
	if envOverride, ok := os.LookupEnv("CONTAINERS_REGISTRIES_CONF"); ok {
		systemContext.SystemRegistriesConfPath = envOverride
		return
	}
	if envOverride, ok := os.LookupEnv("REGISTRIES_CONFIG_PATH"); ok {
		systemContext.SystemRegistriesConfPath = envOverride
		return
	}
}
