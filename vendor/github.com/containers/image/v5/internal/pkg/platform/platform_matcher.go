package platform

// Largely based on
// https://github.com/moby/moby/blob/bc846d2e8fe5538220e0c31e9d0e8446f6fbc022/distribution/cpuinfo_unix.go
// Copyright 2012-2017 Docker, Inc.
//
// https://github.com/containerd/containerd/blob/726dcaea50883e51b2ec6db13caff0e7936b711d/platforms/cpuinfo.go
//    Copyright The containerd Authors.
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//        https://www.apache.org/licenses/LICENSE-2.0
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/containers/image/v5/types"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// For Linux, the kernel has already detected the ABI, ISA and Features.
// So we don't need to access the ARM registers to detect platform information
// by ourselves. We can just parse these information from /proc/cpuinfo
func getCPUInfo(pattern string) (info string, err error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("getCPUInfo for OS %s not implemented", runtime.GOOS)
	}

	cpuinfo, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return "", err
	}
	defer cpuinfo.Close()

	// Start to Parse the Cpuinfo line by line. For SMP SoC, we parse
	// the first core is enough.
	scanner := bufio.NewScanner(cpuinfo)
	for scanner.Scan() {
		newline := scanner.Text()
		list := strings.Split(newline, ":")

		if len(list) > 1 && strings.EqualFold(strings.TrimSpace(list[0]), pattern) {
			return strings.TrimSpace(list[1]), nil
		}
	}

	// Check whether the scanner encountered errors
	err = scanner.Err()
	if err != nil {
		return "", err
	}

	return "", fmt.Errorf("getCPUInfo for pattern: %s not found", pattern)
}

func getCPUVariantWindows(arch string) string {
	// Windows only supports v7 for ARM32 and v8 for ARM64 and so we can use
	// runtime.GOARCH to determine the variants
	var variant string
	switch arch {
	case "arm64":
		variant = "v8"
	case "arm":
		variant = "v7"
	default:
		variant = ""
	}

	return variant
}

func getCPUVariantArm() string {
	variant, err := getCPUInfo("Cpu architecture")
	if err != nil {
		return ""
	}
	// TODO handle RPi Zero mismatch (https://github.com/moby/moby/pull/36121#issuecomment-398328286)

	switch strings.ToLower(variant) {
	case "8", "aarch64":
		variant = "v8"
	case "7", "7m", "?(12)", "?(13)", "?(14)", "?(15)", "?(16)", "?(17)":
		variant = "v7"
	case "6", "6tej":
		variant = "v6"
	case "5", "5t", "5te", "5tej":
		variant = "v5"
	case "4", "4t":
		variant = "v4"
	case "3":
		variant = "v3"
	default:
		variant = ""
	}

	return variant
}

func getCPUVariant(os string, arch string) string {
	if os == "windows" {
		return getCPUVariantWindows(arch)
	}
	if arch == "arm" || arch == "arm64" {
		return getCPUVariantArm()
	}
	return ""
}

// compatibility contains, for a specified architecture, a list of known variants, in the
// order from most capable (most restrictive) to least capable (most compatible).
// Architectures that don’t have variants should not have an entry here.
var compatibility = map[string][]string{
	"arm":   {"v8", "v7", "v6", "v5"},
	"arm64": {"v8"},
}

// WantedPlatforms returns all compatible platforms with the platform specifics possibly overridden by user,
// the most compatible platform is first.
// If some option (arch, os, variant) is not present, a value from current platform is detected.
func WantedPlatforms(ctx *types.SystemContext) ([]imgspecv1.Platform, error) {
	wantedArch := runtime.GOARCH
	wantedVariant := ""
	if ctx != nil && ctx.ArchitectureChoice != "" {
		wantedArch = ctx.ArchitectureChoice
	} else {
		// Only auto-detect the variant if we are using the default architecture.
		// If the user has specified the ArchitectureChoice, don't autodetect, even if
		// ctx.ArchitectureChoice == runtime.GOARCH, because we have no idea whether the runtime.GOARCH
		// value is relevant to the use case, and if we do autodetect a variant,
		// ctx.VariantChoice can't be used to override it back to "".
		wantedVariant = getCPUVariant(runtime.GOOS, runtime.GOARCH)
	}
	if ctx != nil && ctx.VariantChoice != "" {
		wantedVariant = ctx.VariantChoice
	}

	wantedOS := runtime.GOOS
	if ctx != nil && ctx.OSChoice != "" {
		wantedOS = ctx.OSChoice
	}

	var variants []string = nil
	if wantedVariant != "" {
		// If the user requested a specific variant, we'll walk down
		// the list from most to least compatible.
		if compatibility[wantedArch] != nil {
			variantOrder := compatibility[wantedArch]
			for i, v := range variantOrder {
				if wantedVariant == v {
					variants = variantOrder[i:]
					break
				}
			}
		}
		if variants == nil {
			// user wants a variant which we know nothing about - not even compatibility
			variants = []string{wantedVariant}
		}
		// Make sure to have a candidate with an empty variant as well.
		variants = append(variants, "")
	} else {
		// Make sure to have a candidate with an empty variant as well.
		variants = append(variants, "")
		// If available add the entire compatibility matrix for the specific architecture.
		if possibleVariants, ok := compatibility[wantedArch]; ok {
			variants = append(variants, possibleVariants...)
		}
	}

	res := make([]imgspecv1.Platform, 0, len(variants))
	for _, v := range variants {
		res = append(res, imgspecv1.Platform{
			OS:           wantedOS,
			Architecture: wantedArch,
			Variant:      v,
		})
	}
	return res, nil
}

// MatchesPlatform returns true if a platform descriptor from a multi-arch image matches
// an item from the return value of WantedPlatforms.
func MatchesPlatform(image imgspecv1.Platform, wanted imgspecv1.Platform) bool {
	return image.Architecture == wanted.Architecture &&
		image.OS == wanted.OS &&
		image.Variant == wanted.Variant
}
