package entities

import "github.com/containers/podman/v4/libpod/define"

type ListReporter struct {
	Name           string
	Default        bool
	Created        string
	Running        bool
	Starting       bool
	LastUp         string
	Stream         string
	VMType         string
	CPUs           uint64
	Memory         string
	DiskSize       string
	Port           int
	RemoteUsername string
	IdentityPath   string
}

// MachineInfo contains info on the machine host and version info
type MachineInfo struct {
	Host    *MachineHostInfo `json:"Host"`
	Version define.Version   `json:"Version"`
}

// MachineHostInfo contains info on the machine host
type MachineHostInfo struct {
	Arch             string `json:"Arch"`
	CurrentMachine   string `json:"CurrentMachine"`
	DefaultMachine   string `json:"DefaultMachine"`
	EventsDir        string `json:"EventsDir"`
	MachineConfigDir string `json:"MachineConfigDir"`
	MachineImageDir  string `json:"MachineImageDir"`
	MachineState     string `json:"MachineState"`
	NumberOfMachines int    `json:"NumberOfMachines"`
	OS               string `json:"OS"`
	VMType           string `json:"VMType"`
}
