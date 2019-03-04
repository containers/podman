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

type BuildValues struct {
	PodmanCommand
	*buildahcli.BudResults
	*buildahcli.UserNSResults
	*buildahcli.FromAndBudResults
	*buildahcli.NameSpaceResults
	*buildahcli.LayerResults
}

type CpValues struct {
	PodmanCommand
	Extract bool
}
