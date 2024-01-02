package registry

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartAndStopMultipleRegistries(t *testing.T) {
	binary = "../podman-registry"

	registries := []*Registry{}

	registryOptions := &Options{
		PodmanPath: "../../bin/podman",
	}

	// Start registries.
	var errs error
	for i := 0; i < 3; i++ {
		reg, err := StartWithOptions(registryOptions)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		assert.True(t, len(reg.Image) > 0)
		assert.True(t, len(reg.User) > 0)
		assert.True(t, len(reg.Password) > 0)
		assert.True(t, len(reg.Port) > 0)
		registries = append(registries, reg)
	}

	// Stop registries.
	for _, reg := range registries {
		// Make sure we can stop it properly.
		errs = errors.Join(errs, reg.Stop())
		// Stopping an already stopped registry is fine as well.
		errs = errors.Join(errs, reg.Stop())
	}

	require.NoError(t, errs)
}
