package define

var (
	// DefaultInitPath is the default path to the container-init binary
	DefaultInitPath = "/usr/libexec/podman/catatonit"
)

// CtrRemoveTimeout is the default number of seconds to wait after stopping a container
// before sending the kill signal
const CtrRemoveTimeout = 10
