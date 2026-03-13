//go:build !remote && (linux || freebsd)

package generate

import (
	"testing"

	"github.com/containers/podman/v6/libpod"
	"github.com/containers/podman/v6/pkg/specgen"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/require"
)

func TestExtractCDIDevices(t *testing.T) {
	testCases := []struct {
		description          string
		generator            specgen.SpecGenerator
		containerConfDevices []string
		defaultHostDevices   []specs.LinuxDevice
		expectedOptions      []libpod.CtrCreateOption
		expectedGenerator    specgen.SpecGenerator
	}{
		{
			description: "no cdi devices does not update specgen",
			generator: specgen.SpecGenerator{
				ContainerStorageConfig: specgen.ContainerStorageConfig{
					Devices: []specs.LinuxDevice{
						{Path: "/dev/foo"},
					},
				},
			},
			expectedGenerator: specgen.SpecGenerator{
				ContainerStorageConfig: specgen.ContainerStorageConfig{
					Devices: []specs.LinuxDevice{
						{Path: "/dev/foo"},
					},
				},
			},
		},
		{
			description: "cdi device is removed from generator",
			generator: specgen.SpecGenerator{
				ContainerStorageConfig: specgen.ContainerStorageConfig{
					Devices: []specs.LinuxDevice{
						{Path: "example.com/class=device"},
					},
				},
			},
			expectedOptions: []libpod.CtrCreateOption{libpod.WithCDI([]string{"example.com/class=device"})},
			expectedGenerator: specgen.SpecGenerator{
				ContainerStorageConfig: specgen.ContainerStorageConfig{
					Devices: []specs.LinuxDevice{},
				},
			},
		},
		{
			description: "cdi device is removed from generator with existing device",
			generator: specgen.SpecGenerator{
				ContainerStorageConfig: specgen.ContainerStorageConfig{
					Devices: []specs.LinuxDevice{
						{Path: "example.com/class=device"},
						{Path: "/dev/foo"},
					},
				},
			},
			expectedOptions: []libpod.CtrCreateOption{libpod.WithCDI([]string{"example.com/class=device"})},
			expectedGenerator: specgen.SpecGenerator{
				ContainerStorageConfig: specgen.ContainerStorageConfig{
					Devices: []specs.LinuxDevice{
						{Path: "/dev/foo"},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			options := ExtractCDIDevices(&tc.generator)
			require.EqualValues(t, len(tc.expectedOptions), len(options))
			require.EqualValues(t, tc.expectedGenerator, tc.generator)
		})
	}
}
