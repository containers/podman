package shared

import (
	"testing"

	"github.com/containers/libpod/pkg/util"
	"github.com/stretchr/testify/assert"
)

var (
	name      = "foo"
	imageName = "bar"
)

func TestGenerateRunEnvironment(t *testing.T) {
	opts := make(map[string]string)
	opts["opt1"] = "one"
	opts["opt2"] = "two"
	opts["opt3"] = "three"
	envs := GenerateRunEnvironment(name, imageName, opts)
	assert.True(t, util.StringInSlice("OPT1=one", envs))
	assert.True(t, util.StringInSlice("OPT2=two", envs))
	assert.True(t, util.StringInSlice("OPT3=three", envs))
}

func TestGenerateRunEnvironmentNoOpts(t *testing.T) {
	opts := make(map[string]string)
	envs := GenerateRunEnvironment(name, imageName, opts)
	assert.False(t, util.StringInSlice("OPT1=", envs))
	assert.False(t, util.StringInSlice("OPT2=", envs))
	assert.False(t, util.StringInSlice("OPT3=", envs))
}

func TestGenerateRunEnvironmentSingleOpt(t *testing.T) {
	opts := make(map[string]string)
	opts["opt1"] = "one"
	envs := GenerateRunEnvironment(name, imageName, opts)
	assert.True(t, util.StringInSlice("OPT1=one", envs))
	assert.False(t, util.StringInSlice("OPT2=", envs))
	assert.False(t, util.StringInSlice("OPT3=", envs))
}

func TestGenerateRunEnvironmentName(t *testing.T) {
	opts := make(map[string]string)
	envs := GenerateRunEnvironment(name, imageName, opts)
	assert.True(t, util.StringInSlice("NAME=foo", envs))
}

func TestGenerateRunEnvironmentImage(t *testing.T) {
	opts := make(map[string]string)
	envs := GenerateRunEnvironment(name, imageName, opts)
	assert.True(t, util.StringInSlice("IMAGE=bar", envs))
}
