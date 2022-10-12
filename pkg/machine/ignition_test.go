package machine

import (
	"fmt"
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
			t.Error(fmt.Sprintf("Expected %s got %s", test.expected, got))
		}
	}
}
