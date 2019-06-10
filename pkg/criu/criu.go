package criu

import (
	"github.com/checkpoint-restore/go-criu"
)

// MinCriuVersion for Podman at least CRIU 3.11 is required
const MinCriuVersion = 31100

// CheckForCriu uses CRIU's go bindings to check if the CRIU
// binary exists and if it at least the version Podman needs.
func CheckForCriu() bool {
	c := criu.MakeCriu()
	result, err := c.IsCriuAtLeast(MinCriuVersion)
	if err != nil {
		return false
	}
	return result
}
