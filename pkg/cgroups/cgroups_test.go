package cgroups

import (
	"testing"

	"github.com/containers/podman/v3/pkg/rootless"
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

func TestCreated(t *testing.T) {
	// tests only works in rootless mode
	if rootless.IsRootless() {
		return
	}

	var resources spec.LinuxResources
	cgr, err := New("machine.slice", &resources)
	if err != nil {
		t.Error(err)
	}
	if err := cgr.Delete(); err != nil {
		t.Error(err)
	}

	cgr, err = NewSystemd("machine.slice")
	if err != nil {
		t.Error(err)
	}
	if err := cgr.Delete(); err != nil {
		t.Error(err)
	}
}
