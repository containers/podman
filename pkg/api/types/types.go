package types

const (
	// DefaultAPIVersion is the version of the compatible API the server defaults to
	DefaultAPIVersion = "1.40" // See https://docs.docker.com/engine/api/v1.40/

	// MinimalAPIVersion is the minimal required version of the compatible API
	MinimalAPIVersion = "1.24"
)

type APIContextKey int

const (
	DecoderKey APIContextKey = iota
	RuntimeKey
	IdleTrackerKey
	ConnKey
)
