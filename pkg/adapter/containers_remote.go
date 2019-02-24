// +build remoteclient

package adapter

import (
	"encoding/json"
	"github.com/containers/libpod/cmd/podman/shared"

	iopodman "github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/inspect"
)

// Inspect returns an inspect struct from varlink
func (c *Container) Inspect(size bool) (*inspect.ContainerInspectData, error) {
	reply, err := iopodman.ContainerInspectData().Call(c.Runtime.Conn, c.ID())
	if err != nil {
		return nil, err
	}
	data := inspect.ContainerInspectData{}
	if err := json.Unmarshal([]byte(reply), &data); err != nil {
		return nil, err
	}
	return &data, err
}

// ID returns the ID of the container
func (c *Container) ID() string {
	return c.config.ID
}

// GetArtifact returns a container's artifacts
func (c *Container) GetArtifact(name string) ([]byte, error) {
	var data []byte
	reply, err := iopodman.ContainerArtifacts().Call(c.Runtime.Conn, c.ID(), name)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(reply), &data); err != nil {
		return nil, err
	}
	return data, err
}

// Config returns a container's Config ... same as ctr.Config()
func (c *Container) Config() *libpod.ContainerConfig {
	if c.config != nil {
		return c.config
	}
	return c.Runtime.Config(c.ID())
}

// Name returns the name of the container
func (c *Container) Name() string {
	return c.config.Name
}

// BatchContainerOp is wrapper func to mimic shared's function with a similar name meant for libpod
func BatchContainerOp(ctr *Container, opts shared.PsOptions) (shared.BatchContainerStruct, error) {
	// TODO If pod ps ever shows container's sizes, re-enable this code; otherwise it isn't needed
	// and would be a perf hit
	//data, err := ctr.Inspect(true)
	//if err != nil {
	//	return shared.BatchContainerStruct{}, err
	//}
	//
	//size := new(shared.ContainerSize)
	//size.RootFsSize = data.SizeRootFs
	//size.RwSize = data.SizeRw

	bcs := shared.BatchContainerStruct{
		ConConfig:   ctr.config,
		ConState:    ctr.state.State,
		ExitCode:    ctr.state.ExitCode,
		Pid:         ctr.state.PID,
		StartedTime: ctr.state.StartedTime,
		ExitedTime:  ctr.state.FinishedTime,
		//Size: size,
	}
	return bcs, nil
}
