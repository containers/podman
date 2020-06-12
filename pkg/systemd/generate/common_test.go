package generate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterPodFlags(t *testing.T) {

	tests := []struct {
		input []string
	}{
		{[]string{"podman", "pod", "create"}},
		{[]string{"podman", "pod", "create", "--name", "foo"}},
		{[]string{"podman", "pod", "create", "--pod-id-file", "foo"}},
		{[]string{"podman", "run", "--pod", "foo"}},
	}

	for _, test := range tests {
		processed := filterPodFlags(test.input)
		assert.NotContains(t, processed, "--pod-id-file")
		assert.NotContains(t, processed, "--pod")
	}
}
