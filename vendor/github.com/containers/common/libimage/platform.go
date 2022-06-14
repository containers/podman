package libimage

import (
	"context"
	"fmt"
	"runtime"
)

// PlatformPolicy controls the behavior of image-platform matching.
type PlatformPolicy int

const (
	// Only debug log if an image does not match the expected platform.
	PlatformPolicyDefault PlatformPolicy = iota
	// Warn if an image does not match the expected platform.
	PlatformPolicyWarn
)

func toPlatformString(architecture, os, variant string) string {
	if variant == "" {
		return fmt.Sprintf("%s/%s", os, architecture)
	}
	return fmt.Sprintf("%s/%s/%s", os, architecture, variant)
}

// Checks whether the image matches the specified platform.
// Returns
//  * 1) a matching error that can be used for logging (or returning) what does not match
//  * 2) a bool indicating whether architecture, os or variant were set (some callers need that to decide whether they need to throw an error)
//  * 3) a fatal error that occurred prior to check for matches (e.g., storage errors etc.)
func (i *Image) matchesPlatform(ctx context.Context, architecture, os, variant string) (error, bool, error) {
	customPlatform := len(architecture)+len(os)+len(variant) != 0

	if len(architecture) == 0 {
		architecture = runtime.GOARCH
	}
	if len(os) == 0 {
		os = runtime.GOOS
	}

	inspectInfo, err := i.inspectInfo(ctx)
	if err != nil {
		return nil, customPlatform, fmt.Errorf("inspecting image: %w", err)
	}

	matches := true
	switch {
	case architecture != inspectInfo.Architecture:
		matches = false
	case os != inspectInfo.Os:
		matches = false
	case variant != "" && variant != inspectInfo.Variant:
		matches = false
	}

	if matches {
		return nil, customPlatform, nil
	}

	imagePlatform := toPlatformString(inspectInfo.Architecture, inspectInfo.Os, inspectInfo.Variant)
	expectedPlatform := toPlatformString(architecture, os, variant)
	return fmt.Errorf("image platform (%s) does not match the expected platform (%s)", imagePlatform, expectedPlatform), customPlatform, nil
}
