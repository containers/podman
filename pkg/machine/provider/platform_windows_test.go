//go:build windows

package provider

import (
	"testing"

	"github.com/containers/podman/v6/pkg/machine/define"
	"github.com/containers/podman/v6/pkg/machine/hyperv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper to setup mocks and ensure cleanup
func mockPermissions(t *testing.T, hasHyperVPermissions bool) {
	origHyperVPermissionsFunc := hasHyperVPermissionsFunc
	t.Cleanup(func() {
		hasHyperVPermissionsFunc = origHyperVPermissionsFunc
	})

	hasHyperVPermissionsFunc = func() bool { return hasHyperVPermissions }
}

func TestGetByVMType_HyperV(t *testing.T) {
	tests := []struct {
		name                 string
		hasHyperVPermissions bool
		expectError          bool
	}{
		{
			name:                 "WithHyperVPermissions",
			hasHyperVPermissions: true,
			expectError:          false,
		},
		{
			name:                 "WithoutPermissions",
			hasHyperVPermissions: false,
			expectError:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPermissions(t, tt.hasHyperVPermissions)

			provider, err := GetByVMType(define.HyperVVirt)

			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, err.Error(), hyperv.ErrHypervUserNotInAdminGroup.Error())
				assert.Nil(t, provider)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, provider)
				assert.Equal(t, define.HyperVVirt, provider.VMType())
			}
		})
	}
}

func TestGetAll_HyperV_Inclusion(t *testing.T) {
	tests := []struct {
		name                 string
		hasHyperVPermissions bool
		expectHyperV         bool
	}{
		{"WithHyperVPermissions", true, true},
		{"WithoutHyperVPermissions", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPermissions(t, tt.hasHyperVPermissions)

			providers := GetAll()

			// Check for HyperV presence
			hasHyperV := false
			for _, p := range providers {
				if p.VMType() == define.HyperVVirt {
					hasHyperV = true
					break
				}
			}

			assert.Equal(t, tt.expectHyperV, hasHyperV, "Hyper-V provider presence mismatch")

			// WSL should always be present in these scenarios
			hasWSL := false
			for _, p := range providers {
				if p.VMType() == define.WSLVirt {
					hasWSL = true
					break
				}
			}
			assert.True(t, hasWSL, "GetAll should always include WSL provider")
		})
	}
}

func TestGetByVMType_WSL_AlwaysWorks(t *testing.T) {
	provider, err := GetByVMType(define.WSLVirt)
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, define.WSLVirt, provider.VMType())
}

func TestGetByVMType_UnsupportedProvider(t *testing.T) {
	provider, err := GetByVMType(define.QemuVirt)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported virtualization provider")
	assert.Nil(t, provider)
}
