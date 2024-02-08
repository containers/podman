//go:build !remote

package kube

import (
	"testing"

	v1 "github.com/containers/podman/v5/pkg/k8s.io/api/core/v1"
	"github.com/stretchr/testify/assert"
)

func TestVolumeFromEmptyDir(t *testing.T) {
	emptyDirSource := v1.EmptyDirVolumeSource{}
	emptyDirVol, err := VolumeFromEmptyDir(&emptyDirSource, "emptydir")
	assert.NoError(t, err)
	assert.Equal(t, emptyDirVol.Type, KubeVolumeTypeEmptyDir)

	memEmptyDirSource := v1.EmptyDirVolumeSource{
		Medium: v1.StorageMediumMemory,
	}
	memEmptyDirVol, err := VolumeFromEmptyDir(&memEmptyDirSource, "emptydir")
	assert.NoError(t, err)
	assert.Equal(t, memEmptyDirVol.Type, KubeVolumeTypeEmptyDirTmpfs)
}
