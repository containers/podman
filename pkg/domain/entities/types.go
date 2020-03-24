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

type PodDeleteReport struct{ Report }
type PodPruneOptions struct{}
