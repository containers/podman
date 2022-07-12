package entities

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
