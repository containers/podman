//go:build linux
// +build linux

package criu

import (
	"github.com/checkpoint-restore/go-criu/v5"
	"github.com/checkpoint-restore/go-criu/v5/rpc"

	"google.golang.org/protobuf/proto"
)

// MinCriuVersion for Podman at least CRIU 3.11 is required
const MinCriuVersion = 31100

// PodCriuVersion is the version of CRIU needed for
// checkpointing and restoring containers out of and into Pods.
const PodCriuVersion = 31600

// CheckForCriu uses CRIU's go bindings to check if the CRIU
// binary exists and if it at least the version Podman needs.
func CheckForCriu(version int) bool {
	c := criu.MakeCriu()
	result, err := c.IsCriuAtLeast(version)
	if err != nil {
		return false
	}
	return result
}

func GetCriuVestion() (int, error) {
	c := criu.MakeCriu()
	return c.GetCriuVersion()
}

func MemTrack() bool {
	features, err := criu.MakeCriu().FeatureCheck(
		&rpc.CriuFeatures{
			MemTrack: proto.Bool(true),
		},
	)
	if err != nil {
		return false
	}

	if features == nil || features.MemTrack == nil {
		return false
	}

	return *features.MemTrack
}
