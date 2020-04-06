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
//        http://www.apache.org/licenses/LICENSE-2.0
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

var compatibility = map[string][]string{
	"arm":   {"v7", "v6", "v5"},
	"arm64": {"v8"},
}

// Returns all compatible platforms with the platform specifics possibly overriden by user,
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

	var wantedPlatforms []imgspecv1.Platform
	if wantedVariant != "" && compatibility[wantedArch] != nil {
		wantedPlatforms = make([]imgspecv1.Platform, 0, len(compatibility[wantedArch]))
		wantedIndex := -1
		for i, v := range compatibility[wantedArch] {
			if wantedVariant == v {
				wantedIndex = i
				break
			}
		}
		// user wants a variant which we know nothing about - not even compatibility
		if wantedIndex == -1 {
			wantedPlatforms = []imgspecv1.Platform{
				{
					OS:           wantedOS,
					Architecture: wantedArch,
					Variant:      wantedVariant,
				},
			}
		} else {
			for i := wantedIndex; i < len(compatibility[wantedArch]); i++ {
				v := compatibility[wantedArch][i]
				wantedPlatforms = append(wantedPlatforms, imgspecv1.Platform{
					OS:           wantedOS,
					Architecture: wantedArch,
					Variant:      v,
				})
			}
		}
	} else {
		wantedPlatforms = []imgspecv1.Platform{
			{
				OS:           wantedOS,
				Architecture: wantedArch,
				Variant:      wantedVariant,
			},
		}
	}

	return wantedPlatforms, nil
}

func MatchesPlatform(image imgspecv1.Platform, wanted imgspecv1.Platform) bool {
	if image.Architecture != wanted.Architecture {
		return false
	}
	if image.OS != wanted.OS {
		return false
	}

	if wanted.Variant == "" || image.Variant == wanted.Variant {
		return true
	}

	return false
}
