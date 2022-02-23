package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	//"github.com/stretchr/testify/require"
)

func testPropagation(t *testing.T, propagation v1.MountPropagationMode, expected string) {
	dest, options, err := parseMountPath("/to", false, &propagation)
	assert.NoError(t, err)
	assert.Equal(t, dest, "/to")
	assert.Contains(t, options, expected)
}

func TestParseMountPathPropagation(t *testing.T) {
	testPropagation(t, v1.MountPropagationNone, "private")
	testPropagation(t, v1.MountPropagationHostToContainer, "rslave")
	testPropagation(t, v1.MountPropagationBidirectional, "rshared")

	prop := v1.MountPropagationMode("SpaceWave")
	_, _, err := parseMountPath("/to", false, &prop)
	assert.Error(t, err)

	_, options, err := parseMountPath("/to", false, nil)
	assert.NoError(t, err)
	assert.NotContains(t, options, "private")
	assert.NotContains(t, options, "rslave")
	assert.NotContains(t, options, "rshared")
}

func TestParseMountPathRO(t *testing.T) {
	_, options, err := parseMountPath("/to", true, nil)
	assert.NoError(t, err)
	assert.Contains(t, options, "ro")

	_, options, err = parseMountPath("/to", false, nil)
	assert.NoError(t, err)
	assert.NotContains(t, options, "ro")
}
