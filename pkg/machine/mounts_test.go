package machine

import (
	"testing"
)

func TestFSPathToMountName(t *testing.T) {
	tests := []struct {
		given    string
		expected string
	}{
		{
			given:    "/var",
			expected: "var",
		},
		{
			given:    "/var/lib/podman",
			expected: "var-lib-podman",
		},
		{
			given:    "/var/lib/podman/",
			expected: "var-lib-podman",
		},
	}

	for _, test := range tests {
		got := fsPathToMountName(test.given)
		if got != test.expected {
			t.Errorf("Expected %s got %s", test.expected, got)
		}
	}
}

func TestBuildingMountUnits(t *testing.T) {
	tests := []struct {
		mounts           []Mount
		expectedNumUnits int
	}{
		{
			mounts: []Mount{{
				Tag:    "vol0",
				Target: "/Users/linus",
				Type:   "9p",
			}},
			expectedNumUnits: 2,
		},
		{
			mounts: []Mount{{
				Tag:    "vol0",
				Target: "/mnt/test",
				Type:   "9p",
			}},
			expectedNumUnits: 1,
		},
	}

	for _, test := range tests {
		builtUnits := buildMountUnits(test.mounts)
		if len(builtUnits) != test.expectedNumUnits {
			t.Errorf("Expected %d units to be built, got %d", test.expectedNumUnits, len(builtUnits))
		}
	}
}
