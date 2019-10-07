package cliconfig

import (
	buildahcli "github.com/containers/buildah/pkg/cli"
)

type CreateValues struct {
	PodmanCommand
}

type RunValues struct {
	PodmanCommand
}

// PodmanBuildResults represents the results for Podman Build flags
// that are unique to Podman.
type PodmanBuildResults struct {
	SquashAll bool
}

type BuildValues struct {
	PodmanCommand
	*buildahcli.BudResults
	*buildahcli.UserNSResults
	*buildahcli.FromAndBudResults
	*buildahcli.LayerResults
	*buildahcli.NameSpaceResults
	*PodmanBuildResults
}

type CpValues struct {
	PodmanCommand
	Extract bool
	Pause   bool
}
