//go:build amd64 || arm64

package localapi

import (
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/containers/podman/v6/pkg/domain/entities"
	"github.com/containers/podman/v6/pkg/machine/define"
	"github.com/containers/podman/v6/pkg/machine/vmconfigs"
	"github.com/containers/podman/v6/pkg/specgen"
	"github.com/sirupsen/logrus"
)

const machineVolumesDocURL = "https://docs.podman.io/en/latest/markdown/podman-machine-init.1.html#volume"

// WarnIfMachineVolumesUnavailable inspects bind mounts requested via --volume
// and warns if the source paths are not shared with the active Podman machine.
func WarnIfMachineVolumesUnavailable(cfg *entities.PodmanConfig, volumeSpecs []string) {
	if cfg == nil || len(volumeSpecs) == 0 || !cfg.MachineMode {
		return
	}

	parsedURI, err := url.Parse(cfg.URI)
	if err != nil {
		logrus.Debugf("skipping machine volume check, invalid connection URI %q: %v", cfg.URI, err)
		return
	}

	mounts, vmType, err := getMachineMountsAndVMType(cfg.URI, parsedURI)
	if err != nil {
		logrus.Debugf("skipping machine volume check: %v", err)
		return
	}
	if vmType == define.WSLVirt {
		// WSL mounts the drives automatically so a warning would be misleading.
		return
	}

	missing := collectUnsharedHostPaths(volumeSpecs, mounts, vmType)
	if len(missing) == 0 {
		return
	}
	sort.Strings(missing)
	logrus.Warnf("The following bind mount sources are not shared with the Podman machine and may not work: %s. See %s for details on configuring machine volumes.", strings.Join(missing, ", "), machineVolumesDocURL)
}

func collectUnsharedHostPaths(volumeSpecs []string, mounts []*vmconfigs.Mount, vmType define.VMType) []string {
	unshared := []string{}
	seen := make(map[string]struct{})
	for _, spec := range volumeSpecs {
		src, ok := extractBindMountSource(spec)
		if !ok {
			continue
		}
		if _, found := isPathAvailableOnMachine(mounts, vmType, src); found {
			continue
		}
		normalized, err := normalizeVolumeSource(src)
		if err != nil {
			logrus.Debugf("machine volume check: unable to normalize %q: %v", src, err)
			continue
		}
		if _, exists := seen[normalized]; !exists {
			unshared = append(unshared, normalized)
			seen[normalized] = struct{}{}
		}
	}
	return unshared
}

func extractBindMountSource(spec string) (string, bool) {
	parts := specgen.SplitVolumeString(spec)
	if len(parts) <= 1 {
		return "", false
	}
	src := parts[0]
	if len(src) == 0 {
		return "", false
	}
	if strings.HasPrefix(src, "./") {
		resolved, err := filepath.EvalSymlinks(src)
		if err != nil {
			logrus.Debugf("machine volume check: failed to resolve symlinks of %q: %v", src, err)
		} else {
			path, err := filepath.Abs(resolved)
			if err != nil {
				logrus.Debugf("machine volume check: failed to get absolute path of %q: %v", resolved, err)
			} else {
				src = path
			}
		}
	}

	if strings.HasPrefix(src, "/") || strings.HasPrefix(src, ".") || specgen.IsHostWinPath(src) {
		return src, true
	}
	return "", false
}

func normalizeVolumeSource(path string) (string, error) {
	if specgen.IsHostWinPath(path) {
		return filepath.Clean(path), nil
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return absPath, nil
}
