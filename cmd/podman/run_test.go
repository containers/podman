package main

import (
	"testing"

	units "github.com/docker/go-units"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/projectatomic/libpod/pkg/inspect"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli"
)

var (
	cmd         = []string{"podman", "test", "alpine"}
	CLI         *cli.Context
	testCommand = cli.Command{
		Name:   "test",
		Flags:  createFlags,
		Action: testCmd,
	}
)

// generates a mocked ImageData structure based on alpine
func generateAlpineImageData() *inspect.ImageData {
	config := &ociv1.ImageConfig{
		User:         "",
		ExposedPorts: nil,
		Env:          []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
		Entrypoint:   []string{},
		Cmd:          []string{"/bin/sh"},
		Volumes:      nil,
		WorkingDir:   "",
		Labels:       nil,
		StopSignal:   "",
	}

	data := &inspect.ImageData{
		ID:              "e21c333399e0aeedfd70e8827c9fba3f8e9b170ef8a48a29945eb7702bf6aa5f",
		RepoTags:        []string{"docker.io/library/alpine:latest"},
		RepoDigests:     []string{"docker.io/library/alpine@sha256:5cb04fce748f576d7b72a37850641de8bd725365519673c643ef2d14819b42c6"},
		Comment:         "Created:2017-12-01 18:48:48.949613376 +0000",
		Author:          "",
		Architecture:    "amd64",
		Os:              "linux",
		Version:         "17.06.2-ce",
		ContainerConfig: config,
	}
	return data
}

// sets a global CLI
func testCmd(c *cli.Context) error {
	CLI = c
	return nil
}

// creates the mocked cli pointing to our create flags
// global flags like log-level are not implemented
func createCLI() cli.App {
	a := cli.App{
		Commands: []cli.Command{
			testCommand,
		},
	}
	return a
}

func getRuntimeSpec(c *cli.Context) (*spec.Spec, error) {
	/*
		TODO: This test has never worked. Need to install content
		runtime, err := getRuntime(c)
		if err != nil {
		return nil, err
		}
		createConfig, err := parseCreateOpts(c, runtime, "alpine", generateAlpineImageData())
	*/
	createConfig, err := parseCreateOpts(c, nil, "alpine", generateAlpineImageData())
	if err != nil {
		return nil, err
	}
	runtimeSpec, err := createConfigToOCISpec(createConfig)
	if err != nil {
		return nil, err
	}
	return runtimeSpec, nil
}

// TestPIDsLimit verifies the inputed pid-limit is correctly defined in the spec
func TestPIDsLimit(t *testing.T) {
	a := createCLI()
	args := []string{"--pids-limit", "22"}
	a.Run(append(cmd, args...))
	runtimeSpec, err := getRuntimeSpec(CLI)
	if err != nil {
		t.Fatalf(err.Error())
	}
	assert.Equal(t, runtimeSpec.Linux.Resources.Pids.Limit, int64(22))
}

// TestBLKIOWeightDevice verifies the inputed blkio weigh device is correctly defined in the spec
func TestBLKIOWeightDevice(t *testing.T) {
	a := createCLI()
	args := []string{"--blkio-weight-device", "/dev/sda:100"}
	a.Run(append(cmd, args...))
	runtimeSpec, err := getRuntimeSpec(CLI)
	if err != nil {
		t.Fatalf(err.Error())
	}
	assert.Equal(t, *runtimeSpec.Linux.Resources.BlockIO.WeightDevice[0].Weight, uint16(100))
}

// TestMemorySwap verifies that the inputed memory swap is correctly defined in the spec
func TestMemorySwap(t *testing.T) {
	a := createCLI()
	args := []string{"--memory-swap", "45m", "--memory", "40m"}
	a.Run(append(cmd, args...))
	runtimeSpec, err := getRuntimeSpec(CLI)
	if err != nil {
		t.Fatalf(err.Error())
	}
	mem, _ := units.RAMInBytes("45m")
	assert.Equal(t, *runtimeSpec.Linux.Resources.Memory.Swap, mem)
}
