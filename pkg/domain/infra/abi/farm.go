//go:build !remote

package abi

import (
	"context"
	"fmt"
	"strings"

	"github.com/containers/buildah/pkg/parse"
	lplatform "github.com/containers/common/libimage/platform"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/emulation"
)

// FarmNodeName returns the local engine's name.
func (ir *ImageEngine) FarmNodeName(ctx context.Context) string {
	return entities.LocalFarmImageBuilderName
}

// FarmNodeDriver returns a description of the local image builder driver
func (ir *ImageEngine) FarmNodeDriver(ctx context.Context) string {
	return entities.LocalFarmImageBuilderDriver
}

func (ir *ImageEngine) fetchInfo(_ context.Context) (os, arch, variant string, nativePlatforms []string, emulatedPlatforms []string, err error) {
	nativePlatform := parse.DefaultPlatform()
	platform := strings.SplitN(nativePlatform, "/", 3)
	switch len(platform) {
	case 0, 1:
		return "", "", "", nil, nil, fmt.Errorf("unparsable default platform %q", nativePlatform)
	case 2:
		os, arch = platform[0], platform[1]
	case 3:
		os, arch, variant = platform[0], platform[1], platform[2]
	}
	os, arch, variant = lplatform.Normalize(os, arch, variant)
	nativePlatform = os + "/" + arch
	if variant != "" {
		nativePlatform += "/" + variant
	}
	emulatedPlatforms = emulation.Registered()
	return os, arch, variant, append([]string{}, nativePlatform), emulatedPlatforms, nil
}

// FarmNodeInspect returns information about the remote engines in the farm
func (ir *ImageEngine) FarmNodeInspect(ctx context.Context) (*entities.FarmInspectReport, error) {
	ir.platforms.Do(func() {
		ir.os, ir.arch, ir.variant, ir.nativePlatforms, ir.emulatedPlatforms, ir.platformsErr = ir.fetchInfo(ctx)
	})
	return &entities.FarmInspectReport{NativePlatforms: ir.nativePlatforms,
		EmulatedPlatforms: ir.emulatedPlatforms,
		OS:                ir.os,
		Arch:              ir.arch,
		Variant:           ir.variant}, ir.platformsErr
}
