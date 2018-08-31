package image

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecompose(t *testing.T) {
	const digestSuffix = "@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	for _, c := range []struct {
		input                          string
		transport, registry, name, tag string
		isTagged, hasRegistry          bool
		assembled                      string
		assembledWithTransport         string
	}{
		{"#", "", "", "", "", false, false, "", ""}, // Entirely invalid input
		{ // Fully qualified docker.io, name-only input
			"docker.io/library/busybox", "docker://", "docker.io", "library/busybox", "latest", false, true,
			"docker.io/library/busybox:latest", "docker://docker.io/library/busybox:latest",
		},
		{ // Fully qualified example.com, name-only input
			"example.com/ns/busybox", "docker://", "example.com", "ns/busybox", "latest", false, true,
			"example.com/ns/busybox:latest", "docker://example.com/ns/busybox:latest",
		},
		{ // Unqualified single-name input
			"busybox", "docker://", "", "busybox", "latest", false, false,
			"busybox:latest", "docker://busybox:latest",
		},
		{ // Unqualified namespaced input
			"ns/busybox", "docker://", "", "ns/busybox", "latest", false, false,
			"ns/busybox:latest", "docker://ns/busybox:latest",
		},
		{ // name:tag
			"example.com/ns/busybox:notlatest", "docker://", "example.com", "ns/busybox", "notlatest", true, true,
			"example.com/ns/busybox:notlatest", "docker://example.com/ns/busybox:notlatest",
		},
		{ // name@digest
			// FIXME? .tag == "none"
			"example.com/ns/busybox" + digestSuffix, "docker://", "example.com", "ns/busybox", "none", false, true,
			// FIXME: this drops the digest and replaces it with an incorrect tag.
			"example.com/ns/busybox:none", "docker://example.com/ns/busybox:none",
		},
		{ // name:tag@digest
			"example.com/ns/busybox:notlatest" + digestSuffix, "docker://", "example.com", "ns/busybox", "notlatest", true, true,
			// FIXME: This drops the digest
			"example.com/ns/busybox:notlatest", "docker://example.com/ns/busybox:notlatest",
		},
	} {
		parts, err := decompose(c.input)
		if c.transport == "" {
			assert.Error(t, err, c.input)
		} else {
			assert.NoError(t, err, c.input)
			assert.Equal(t, c.transport, parts.transport, c.input)
			assert.Equal(t, c.registry, parts.registry, c.input)
			assert.Equal(t, c.name, parts.name, c.input)
			assert.Equal(t, c.tag, parts.tag, c.input)
			assert.Equal(t, c.isTagged, parts.isTagged, c.input)
			assert.Equal(t, c.hasRegistry, parts.hasRegistry, c.input)
			assert.Equal(t, c.assembled, parts.assemble(), c.input)
			assert.Equal(t, c.assembledWithTransport, parts.assembleWithTransport(), c.input)
		}
	}
}
