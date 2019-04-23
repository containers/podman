package createconfig

import (
	"reflect"
	"testing"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
)

func TestGetVolumeMountsOneVolume(t *testing.T) {
	data := spec.Mount{
		Destination: "/foobar",
		Type:        "bind",
		Source:      "/tmp",
		Options:     []string{"ro"},
	}
	config := CreateConfig{
		Volumes: []string{"/tmp:/foobar:ro"},
	}
	specMount, _, err := config.getVolumeMounts()
	assert.NoError(t, err)
	assert.True(t, reflect.DeepEqual(data, specMount[data.Destination]))
}

func TestGetTmpfsMounts(t *testing.T) {
	data := spec.Mount{
		Destination: "/homer",
		Type:        "tmpfs",
		Source:      "tmpfs",
		Options:     []string{"rw", "size=787448k", "mode=1777"},
	}
	config := CreateConfig{
		Tmpfs: []string{"/homer:rw,size=787448k,mode=1777"},
	}
	tmpfsMount, err := config.getTmpfsMounts()
	assert.NoError(t, err)
	assert.True(t, reflect.DeepEqual(data, tmpfsMount[data.Destination]))
}
