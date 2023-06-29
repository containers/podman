//go:build !linux && !remote

package system

// Currently, we only need servicereaper on Linux to support slirp4netns.
func maybeStartServiceReaper() {
}
