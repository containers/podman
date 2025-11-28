//go:build darwin

package specgen

import "path/filepath"

// ResolveVolumeSourcePath follows symlinks for absolute host paths so they
// line up with the actual on-disk location (e.g. /tmp -> /private/tmp).
// If resolution fails, the original path is returned unchanged.
func ResolveVolumeSourcePath(src string) string {
	resolved, err := filepath.EvalSymlinks(src)
	if err != nil {
		return src
	}
	return resolved
}
