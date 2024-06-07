package provider

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/blang/semver/v4"
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

func GetAll(_ bool) ([]vmconfigs.VMProvider, error) {
	return []vmconfigs.VMProvider{
		new(applehv.AppleHVStubber),
		new(libkrun.LibKrunStubber),
	}, nil
}

// SupportedProviders returns the providers that are supported on the host operating system
func SupportedProviders() []define.VMType {
	supported := []define.VMType{define.AppleHvVirt}
	if runtime.GOARCH == "arm64" {
		return append(supported, define.LibKrun)
	}
	return supported
}

// InstalledProviders returns the supported providers that are installed on the host
func InstalledProviders() ([]define.VMType, error) {
	installed := []define.VMType{}

	appleHvInstalled, err := appleHvInstalled()
	if err != nil {
		return nil, err
	}
	if appleHvInstalled {
		installed = append(installed, define.AppleHvVirt)
	}

	libKrunInstalled, err := libKrunInstalled()
	if err != nil {
		return nil, err
	}
	if libKrunInstalled {
		installed = append(installed, define.LibKrun)
	}

	return installed, nil
}

func appleHvInstalled() (bool, error) {
	var outBuf bytes.Buffer
	// Apple's Virtualization.Framework is only supported on MacOS 11.0+,
	// but to use EFI MacOS 13.0+ is required
	expectedVer, err := semver.Make("13.0.0")
	if err != nil {
		return false, err
	}

	cmd := exec.Command("sw_vers", "--productVersion")
	cmd.Stdout = &outBuf
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("unable to check current macOS version using `sw_vers --productVersion`: %s", err)
	}

	// the output will be in the format of MAJOR.MINOR.PATCH
	output := strings.TrimSuffix(outBuf.String(), "\n")
	currentVer, err := semver.Make(output)
	if err != nil {
		return false, err
	}

	return currentVer.GTE(expectedVer), nil
}

func libKrunInstalled() (bool, error) {
	if runtime.GOARCH != "arm64" {
		return false, nil
	}

	// need to verify that krunkit, virglrenderer, and libkrun-efi are installed
	cfg, err := config.Default()
	if err != nil {
		return false, err
	}

	_, err = cfg.FindHelperBinary("krunkit", false)
	return err == nil, nil
}

// HasPermsForProvider returns whether the host operating system has the proper permissions to use the given provider
func HasPermsForProvider(provider define.VMType) bool {
	// there are no permissions required for AppleHV or LibKrun
	return provider == define.AppleHvVirt || provider == define.LibKrun
}
