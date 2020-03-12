package entities

type Container struct {
	IdOrNamed
}

type Volume struct {
	Identifier
}

type Report struct {
	Id  []string
	Err map[string]error
}

type ContainerDeleteOptions struct{}
type ContainerDeleteReport struct{ Report }
type ContainerPruneReport struct{ Report }

type PodDeleteReport struct{ Report }
type PodPruneOptions struct{}
type PodPruneReport struct{ Report }
type VolumeDeleteOptions struct{}
type VolumeDeleteReport struct{ Report }
type VolumePruneReport struct{ Report }
