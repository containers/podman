//go:build !remote
// +build !remote

package abi

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/containers/buildah/pkg/parse"
	lplatform "github.com/containers/common/libimage/platform"
	istorage "github.com/containers/image/v5/storage"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/emulation"
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
		return "", "", "", nil, nil, fmt.Errorf("unparseable default platform %q", nativePlatform)
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

// PullToFile pulls the image from the remote engine and saves it to a file,
// returning a string-format reference which can be parsed by containers/image.
func (ir *ImageEngine) PullToFile(ctx context.Context, options entities.PullToFileOptions) (reference string, err error) {
	saveOptions := entities.ImageSaveOptions{
		Format: options.SaveFormat,
		Output: options.SaveFile,
	}
	if err := ir.Save(ctx, options.ImageID, nil, saveOptions); err != nil {
		return "", fmt.Errorf("saving image %q: %w", options.ImageID, err)
	}
	return options.SaveFormat + ":" + options.SaveFile, nil
}

// PullToFile pulls the image from the remote engine and saves it to the local
// engine passed in via options, returning a string-format reference which can
// be parsed by containers/image.
func (ir *ImageEngine) PullToLocal(ctx context.Context, options entities.PullToLocalOptions) (reference string, err error) {
	destination := options.Destination
	if destination == nil {
		return "", fmt.Errorf("destination not given, cannot pull image %q", options.ImageID)
	}

	// Check if the image is already present at destination
	var br *entities.BoolReport
	br, err = destination.Exists(ctx, options.ImageID)
	if err != nil {
		return "", err
	}
	if br.Value {
		return istorage.Transport.Name() + ":" + options.ImageID, nil
	}

	tempFile, err := os.CreateTemp("", "")
	if err != nil {
		return "", err
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	saveOptions := entities.ImageSaveOptions{
		Format: options.SaveFormat,
		Output: tempFile.Name(),
	}
	// Save image built on builder in a temp file
	if err := ir.Save(ctx, options.ImageID, nil, saveOptions); err != nil {
		return "", fmt.Errorf("saving image %q: %w", options.ImageID, err)
	}

	// Load the image saved in tempFile into the local engine
	loadOptions := entities.ImageLoadOptions{
		Input: tempFile.Name(),
	}

	_, err = destination.Load(ctx, loadOptions)
	if err != nil {
		return "", err
	}

	return istorage.Transport.Name() + ":" + options.ImageID, nil
}
