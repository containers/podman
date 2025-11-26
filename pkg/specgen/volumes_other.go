//go:build !darwin

package specgen

// ResolveVolumeSourcePath is a no-op on non-macOS platforms.
func ResolveVolumeSourcePath(src string) string {
	return src
}
