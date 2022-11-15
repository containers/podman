//go:build linux && !remote

package system

import (
	"github.com/containers/common/pkg/servicereaper"
)

// Currently, we only need servicereaper on Linux to support slirp4netns.
func maybeStartServiceReaper() {
	servicereaper.Start()
}
