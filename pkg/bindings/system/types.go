package system

//go:generate go run ../generator/generator.go EventsOptions
// EventsOptions are optional options for monitoring events
type EventsOptions struct {
	Filters map[string][]string
	Since   *string
	Stream  *bool
	Until   *string
}

//go:generate go run ../generator/generator.go PruneOptions
// PruneOptions are optional options for pruning
type PruneOptions struct {
	All     *bool
	Filters map[string][]string
	Volumes *bool
}

//go:generate go run ../generator/generator.go VersionOptions
// VersionOptions are optional options for getting version info
type VersionOptions struct {
}

//go:generate go run ../generator/generator.go DiskOptions
// DiskOptions are optional options for getting storage consumption
type DiskOptions struct {
}

//go:generate go run ../generator/generator.go InfoOptions
// InfoOptions are optional options for getting info
// about libpod
type InfoOptions struct {
}
