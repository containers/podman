package shared

import (
	"strings"
	"testing"

	"github.com/containers/libpod/pkg/util"
	"github.com/stretchr/testify/assert"
)

var (
	name      = "foo"
	imageName = "bar"
)

func TestGenerateCommand(t *testing.T) {
	inputCommand := "docker run -it --name NAME -e NAME=NAME -e IMAGE=IMAGE IMAGE echo install"
	correctCommand := "podman run -it --name bar -e NAME=bar -e IMAGE=foo foo echo install"
	newCommand := GenerateCommand(inputCommand, "foo", "bar")
	assert.Equal(t, correctCommand, strings.Join(newCommand, " "))
}

func TestGenerateCommandPath(t *testing.T) {
	inputCommand := "/usr/bin/docker run -it --name NAME -e NAME=NAME -e IMAGE=IMAGE IMAGE echo install"
	correctCommand := "podman run -it --name bar -e NAME=bar -e IMAGE=foo foo echo install"
	newCommand := GenerateCommand(inputCommand, "foo", "bar")
	assert.Equal(t, correctCommand, strings.Join(newCommand, " "))
}

func TestGenerateCommandNoSetName(t *testing.T) {
	inputCommand := "docker run -it --name NAME -e NAME=NAME -e IMAGE=IMAGE IMAGE echo install"
	correctCommand := "podman run -it --name foo -e NAME=foo -e IMAGE=foo foo echo install"
	newCommand := GenerateCommand(inputCommand, "foo", "")
	assert.Equal(t, correctCommand, strings.Join(newCommand, " "))
}

func TestGenerateCommandNoName(t *testing.T) {
	inputCommand := "docker run -it  -e IMAGE=IMAGE IMAGE echo install"
	correctCommand := "podman run -it  -e IMAGE=foo foo echo install"
	newCommand := GenerateCommand(inputCommand, "foo", "")
	assert.Equal(t, correctCommand, strings.Join(newCommand, " "))
}

func TestGenerateCommandAlreadyPodman(t *testing.T) {
	inputCommand := "podman run -it --name NAME -e NAME=NAME -e IMAGE=IMAGE IMAGE echo install"
	correctCommand := "podman run -it --name bar -e NAME=bar -e IMAGE=foo foo echo install"
	newCommand := GenerateCommand(inputCommand, "foo", "bar")
	assert.Equal(t, correctCommand, strings.Join(newCommand, " "))
}

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
