package image

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecompose(t *testing.T) {
	const digestSuffix = "@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	for _, c := range []struct {
		input                 string
		registry, name, tag   string
		isTagged, hasRegistry bool
		assembled             string
	}{
		{"#", "", "", "", false, false, ""}, // Entirely invalid input
		{ // Fully qualified docker.io, name-only input
			"docker.io/library/busybox", "docker.io", "library/busybox", "latest", false, true,
			"docker.io/library/busybox:latest",
		},
		{ // Fully qualified example.com, name-only input
			"example.com/ns/busybox", "example.com", "ns/busybox", "latest", false, true,
			"example.com/ns/busybox:latest",
		},
		{ // Unqualified single-name input
			"busybox", "", "busybox", "latest", false, false,
			"busybox:latest",
		},
		{ // Unqualified namespaced input
			"ns/busybox", "", "ns/busybox", "latest", false, false,
			"ns/busybox:latest",
		},
		{ // name:tag
			"example.com/ns/busybox:notlatest", "example.com", "ns/busybox", "notlatest", true, true,
			"example.com/ns/busybox:notlatest",
		},
		{ // name@digest
			// FIXME? .tag == "none"
			"example.com/ns/busybox" + digestSuffix, "example.com", "ns/busybox", "none", false, true,
			// FIXME: this drops the digest and replaces it with an incorrect tag.
			"example.com/ns/busybox:none",
		},
		{ // name:tag@digest
			"example.com/ns/busybox:notlatest" + digestSuffix, "example.com", "ns/busybox", "notlatest", true, true,
			// FIXME: This drops the digest
			"example.com/ns/busybox:notlatest",
		},
	} {
		parts, err := decompose(c.input)
		if c.assembled == "" {
			assert.Error(t, err, c.input)
		} else {
			assert.NoError(t, err, c.input)
			assert.Equal(t, c.registry, parts.registry, c.input)
			assert.Equal(t, c.name, parts.name, c.input)
			assert.Equal(t, c.tag, parts.tag, c.input)
			assert.Equal(t, c.isTagged, parts.isTagged, c.input)
			assert.Equal(t, c.hasRegistry, parts.hasRegistry, c.input)
			assert.Equal(t, c.assembled, parts.assemble(), c.input)
		}
	}
}

func TestImagePartsReferenceWithRegistry(t *testing.T) {
	const digestSuffix = "@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	for _, c := range []struct {
		input                     string
		withDocker, withNonDocker string
	}{
		{"example.com/ns/busybox", "", ""},                                                                            // Fully-qualified input is invalid.
		{"busybox", "docker.io/library/busybox", "example.com/busybox"},                                               // Single-name input
		{"ns/busybox", "docker.io/ns/busybox", "example.com/ns/busybox"},                                              // Namespaced input
		{"ns/busybox:notlatest", "docker.io/ns/busybox:notlatest", "example.com/ns/busybox:notlatest"},                // name:tag
		{"ns/busybox" + digestSuffix, "docker.io/ns/busybox" + digestSuffix, "example.com/ns/busybox" + digestSuffix}, // name@digest
		{ // name:tag@digest
			"ns/busybox:notlatest" + digestSuffix,
			"docker.io/ns/busybox:notlatest" + digestSuffix, "example.com/ns/busybox:notlatest" + digestSuffix,
		},
	} {
		parts, err := decompose(c.input)
		require.NoError(t, err)
		if c.withDocker == "" {
			_, err := parts.referenceWithRegistry("docker.io")
			assert.Error(t, err, c.input)
			_, err = parts.referenceWithRegistry("example.com")
			assert.Error(t, err, c.input)
		} else {
			ref, err := parts.referenceWithRegistry("docker.io")
			require.NoError(t, err, c.input)
			assert.Equal(t, c.withDocker, ref.String())
			ref, err = parts.referenceWithRegistry("example.com")
			require.NoError(t, err, c.input)
			assert.Equal(t, c.withNonDocker, ref.String())
		}
	}

	// Invalid registry value
	parts, err := decompose("busybox")
	require.NoError(t, err)
	_, err = parts.referenceWithRegistry("invalid@domain")
	assert.Error(t, err)
}
