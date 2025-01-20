package provider

import (
	"runtime"
	"testing"

	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/stretchr/testify/assert"
)

func TestSupportedProviders(t *testing.T) {
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			assert.Equal(t, []define.VMType{define.AppleHvVirt, define.LibKrun}, SupportedProviders())
		} else {
			assert.Equal(t, []define.VMType{define.AppleHvVirt}, SupportedProviders())
		}
	case "windows":
		assert.Equal(t, []define.VMType{define.WSLVirt, define.HyperVVirt}, SupportedProviders())
	case "linux":
		assert.Equal(t, []define.VMType{define.QemuVirt}, SupportedProviders())
	}
}

func TestInstalledProviders(t *testing.T) {
	installed, err := InstalledProviders()
	assert.NoError(t, err)
	switch runtime.GOOS {
	case "darwin":
		// TODO: need to verify if an arm64 machine reports {applehv, libkrun}
		assert.Equal(t, []define.VMType{define.AppleHvVirt}, installed)
	case "windows":
		provider, err := Get()
		assert.NoError(t, err)
		assert.Contains(t, installed, provider)
	case "linux":
		assert.Equal(t, []define.VMType{define.QemuVirt}, installed)
	}
}

func TestHasPermsForProvider(t *testing.T) {
	provider, err := Get()
	assert.NoError(t, err)
	assert.True(t, HasPermsForProvider(provider.VMType()))
}

func TestHasBadPerms(t *testing.T) {
	switch runtime.GOOS {
	case "darwin":
		assert.False(t, HasPermsForProvider(define.QemuVirt))
	case "windows":
		assert.False(t, HasPermsForProvider(define.QemuVirt))
	case "linux":
		assert.False(t, HasPermsForProvider(define.AppleHvVirt))
	}
}

func TestBadSupportedProviders(t *testing.T) {
	switch runtime.GOOS {
	case "darwin":
		assert.NotEqual(t, []define.VMType{define.QemuVirt}, SupportedProviders())
		if runtime.GOARCH != "arm64" {
			assert.NotEqual(t, []define.VMType{define.AppleHvVirt, define.LibKrun}, SupportedProviders())
		}
	case "windows":
		assert.NotEqual(t, []define.VMType{define.QemuVirt}, SupportedProviders())
	case "linux":
		assert.NotEqual(t, []define.VMType{define.AppleHvVirt}, SupportedProviders())
	}
}

func TestBadInstalledProviders(t *testing.T) {
	installed, err := InstalledProviders()
	assert.NoError(t, err)
	switch runtime.GOOS {
	case "darwin":
		assert.NotEqual(t, []define.VMType{define.QemuVirt}, installed)
		if runtime.GOARCH != "arm64" {
			assert.NotEqual(t, []define.VMType{define.AppleHvVirt, define.LibKrun}, installed)
		}
	case "windows":
		assert.NotContains(t, installed, define.QemuVirt)
	case "linux":
		assert.NotEqual(t, []define.VMType{define.AppleHvVirt}, installed)
	}
}
