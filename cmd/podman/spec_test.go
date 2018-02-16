package main

import (
	"reflect"
	"testing"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
)

func TestCreateConfig_GetVolumeMounts(t *testing.T) {
	data := spec.Mount{
		Destination: "/foobar",
		Type:        "bind",
		Source:      "foobar",
		Options:     []string{"ro", "rbind", "private"},
	}
	config := createConfig{
		Volumes: []string{"foobar:/foobar:ro"},
	}
	specMount, err := config.GetVolumeMounts([]spec.Mount{})
	assert.NoError(t, err)
	assert.True(t, reflect.DeepEqual(data, specMount[0]))
}

func TestCreateConfig_GetAnnotations(t *testing.T) {
	config := createConfig{}
	annotations := config.GetAnnotations()
	assert.True(t, reflect.DeepEqual("sandbox", annotations["io.kubernetes.cri-o.ContainerType"]))
}

func TestCreateConfig_GetTmpfsMounts(t *testing.T) {
	data := spec.Mount{
		Destination: "/homer",
		Type:        "tmpfs",
		Source:      "tmpfs",
		Options:     []string{"rw", "size=787448k", "mode=1777"},
	}
	config := createConfig{
		Tmpfs: []string{"/homer:rw,size=787448k,mode=1777"},
	}
	tmpfsMount := config.GetTmpfsMounts()
	assert.True(t, reflect.DeepEqual(data, tmpfsMount[0]))

}
