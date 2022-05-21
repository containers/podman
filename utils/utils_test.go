//go:build linux || darwin
// +build linux darwin

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCgroupProcess(t *testing.T) {
	val, err := getCgroupProcess("testdata/cgroup.root", true)
	assert.Nil(t, err)
	assert.Equal(t, "/", val)

	_, err = getCgroupProcess("testdata/cgroup.root", false)
	assert.NotNil(t, err)

	val, err = getCgroupProcess("testdata/cgroup.other", true)
	assert.Nil(t, err)
	assert.Equal(t, "/other", val)

	_, err = getCgroupProcess("testdata/cgroup.empty", true)
	assert.NotNil(t, err)
}
