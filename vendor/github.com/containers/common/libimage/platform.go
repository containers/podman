package libimage

import (
	"context"
	"fmt"
	"runtime"

	"github.com/containerd/containerd/platforms"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

// PlatformPolicy controls the behavior of image-platform matching.
type PlatformPolicy int

const (
	// Only debug log if an image does not match the expected platform.
	PlatformPolicyDefault PlatformPolicy = iota
	// Warn if an image does not match the expected platform.
	PlatformPolicyWarn
)

// NormalizePlatform normalizes (according to the OCI spec) the specified os,
// arch and variant.  If left empty, the individual item will be normalized.
func NormalizePlatform(rawOS, rawArch, rawVariant string) (os, arch, variant string) {
	platformSpec := v1.Platform{
		OS:           rawOS,
		Architecture: rawArch,
		Variant:      rawVariant,
	}
	normalizedSpec := platforms.Normalize(platformSpec)
	if normalizedSpec.Variant == "" && rawVariant != "" {
		normalizedSpec.Variant = rawVariant
	}
	rawPlatform := toPlatformString(normalizedSpec.OS, normalizedSpec.Architecture, normalizedSpec.Variant)
	normalizedPlatform, err := platforms.Parse(rawPlatform)
	if err != nil {
		logrus.Debugf("Error normalizing platform: %v", err)
		return rawOS, rawArch, rawVariant
	}
	logrus.Debugf("Normalized platform %s to %s", rawPlatform, normalizedPlatform)
	os = rawOS
	if rawOS != "" {
		os = normalizedPlatform.OS
	}
	arch = rawArch
	if rawArch != "" {
		arch = normalizedPlatform.Architecture
	}
	variant = rawVariant
	if rawVariant != "" || (rawVariant == "" && normalizedPlatform.Variant != "") {
		variant = normalizedPlatform.Variant
	}
	return os, arch, variant
}

func toPlatformString(os, arch, variant string) string {
	if os == "" {
		os = runtime.GOOS
	}
	if arch == "" {
		arch = runtime.GOARCH
	}
	if variant == "" {
		return fmt.Sprintf("%s/%s", os, arch)
	}
	return fmt.Sprintf("%s/%s/%s", os, arch, variant)
}

// Checks whether the image matches the specified platform.
// Returns
//   - 1) a matching error that can be used for logging (or returning) what does not match
//   - 2) a bool indicating whether architecture, os or variant were set (some callers need that to decide whether they need to throw an error)
//   - 3) a fatal error that occurred prior to check for matches (e.g., storage errors etc.)
func (i *Image) matchesPlatform(ctx context.Context, os, arch, variant string) (error, bool, error) {
	if err := i.isCorrupted(""); err != nil {
		return err, false, nil
	}
	inspectInfo, err := i.inspectInfo(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("inspecting image: %w", err)
	}

	customPlatform := len(os)+len(arch)+len(variant) != 0

	expected, err := platforms.Parse(toPlatformString(os, arch, variant))
	if err != nil {
		return nil, false, fmt.Errorf("parsing host platform: %v", err)
	}
	fromImage, err := platforms.Parse(toPlatformString(inspectInfo.Os, inspectInfo.Architecture, inspectInfo.Variant))
	if err != nil {
		return nil, false, fmt.Errorf("parsing image platform: %v", err)
	}

	if platforms.NewMatcher(expected).Match(fromImage) {
		return nil, customPlatform, nil
	}

	return fmt.Errorf("image platform (%s) does not match the expected platform (%s)", platforms.Format(fromImage), platforms.Format(expected)), customPlatform, nil
}
