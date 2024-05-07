package provider

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v5/pkg/machine/applehv"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/libkrun"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
	"github.com/sirupsen/logrus"
)

func Get() (vmconfigs.VMProvider, error) {
	cfg, err := config.Default()
	if err != nil {
		return nil, err
	}
	provider := cfg.Machine.Provider
	if providerOverride, found := os.LookupEnv("CONTAINERS_MACHINE_PROVIDER"); found {
		provider = providerOverride
	}
	resolvedVMType, err := define.ParseVMType(provider, define.AppleHvVirt)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("Using Podman machine with `%s` virtualization provider", resolvedVMType.String())
	switch resolvedVMType {
	case define.AppleHvVirt:
		return new(applehv.AppleHVStubber), nil
	case define.LibKrun:
		return new(libkrun.LibKrunStubber), nil
	default:
		return nil, fmt.Errorf("unsupported virtualization provider: `%s`", resolvedVMType.String())
	}
}

// SupportedProviders returns the providers that are supported on the host operating system
func SupportedProviders() []define.VMType {
	return []define.VMType{define.AppleHvVirt}
}

// InstalledProviders returns the supported providers that are installed on the host
func InstalledProviders() ([]define.VMType, error) {
	var outBuf bytes.Buffer
	// Apple's Virtualization.Framework is only supported on MacOS 11.0+
	const SupportedMacOSVersion = 11

	cmd := exec.Command("sw_vers", "--productVersion")
	cmd.Stdout = &outBuf
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("unable to check current macOS version using `sw_vers --productVersion`: %s", err)
	}

	// the output will be in the format of MAJOR.MINOR.PATCH
	output := outBuf.String()
	idx := strings.Index(output, ".")
	if idx < 0 {
		return nil, errors.New("invalid output provided by sw_vers --productVersion")
	}
	majorString := output[:idx]
	majorInt, err := strconv.Atoi(majorString)
	if err != nil {
		return nil, err
	}

	if majorInt >= SupportedMacOSVersion {
		return []define.VMType{define.AppleHvVirt}, nil
	}

	return []define.VMType{}, nil
}

// HasPermsForProvider returns whether the host operating system has the proper permissions to use the given provider
func HasPermsForProvider(provider define.VMType) bool {
	// there are no permissions required for AppleHV
	return provider == define.AppleHvVirt
}
