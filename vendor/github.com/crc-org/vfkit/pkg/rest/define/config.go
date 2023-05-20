package define

// InspectResponse is used when responding to a request for
// information about the virtual machine
type InspectResponse struct {
	CPUs   uint   `json:"cpus"`
	Memory uint64 `json:"memory"`
	// Devices []config.VirtioDevice `json:"devices"`
}

// VMState can be used to describe the current state of a VM
// as well as used to request a state change
type VMState struct {
	State string `json:"state"`
}

type StateChange string

const (
	Resume   StateChange = "Resume"
	Pause    StateChange = "Pause"
	Stop     StateChange = "Stop"
	HardStop StateChange = "HardStop"
)
