package pods

//go:generate go run ../generator/generator.go CreateOptions
// CreateOptions are optional options for creating pods
type CreateOptions struct {
}

//go:generate go run ../generator/generator.go InspectOptions
// InspectOptions are optional options for inspecting pods
type InspectOptions struct {
}

//go:generate go run ../generator/generator.go KillOptions
// KillOptions are optional options for killing pods
type KillOptions struct {
	Signal *string
}

//go:generate go run ../generator/generator.go PauseOptions
// PauseOptions are optional options for pausing pods
type PauseOptions struct {
}

//go:generate go run ../generator/generator.go PruneOptions
// PruneOptions are optional options for pruning pods
type PruneOptions struct {
}

//go:generate go run ../generator/generator.go ListOptions
// ListOptions are optional options for listing pods
type ListOptions struct {
	Filters map[string][]string
}

//go:generate go run ../generator/generator.go RestartOptions
// RestartOptions are optional options for restarting pods
type RestartOptions struct {
}

//go:generate go run ../generator/generator.go StartOptions
// StartOptions are optional options for starting pods
type StartOptions struct {
}

//go:generate go run ../generator/generator.go StopOptions
// StopOptions are optional options for stopping pods
type StopOptions struct {
	Timeout *int
}

//go:generate go run ../generator/generator.go TopOptions
// TopOptions are optional options for getting top on pods
type TopOptions struct {
	Descriptors []string
}

//go:generate go run ../generator/generator.go UnpauseOptions
// UnpauseOptions are optional options for unpausinging pods
type UnpauseOptions struct {
}

//go:generate go run ../generator/generator.go StatsOptions
// StatsOptions are optional options for getting stats of pods
type StatsOptions struct {
	All *bool
}

//go:generate go run ../generator/generator.go RemoveOptions
// RemoveOptions are optional options for removing pods
type RemoveOptions struct {
	Force   *bool
	Timeout *uint
}

//go:generate go run ../generator/generator.go ExistsOptions
// ExistsOptions are optional options for checking if a pod exists
type ExistsOptions struct {
}
