package entities

import (
	"github.com/containers/common/pkg/config"
	"github.com/spf13/pflag"
)

// EngineMode is the connection type podman is using to access libpod
type EngineMode string

// EngineSetup calls out whether a "normal" or specialized engine should be created
type EngineSetup string

const (
	ABIMode    = EngineMode("abi")
	TunnelMode = EngineMode("tunnel")

	MigrateMode  = EngineSetup("migrate")
	NoFDsMode    = EngineSetup("disablefds")
	NormalMode   = EngineSetup("normal")
	RenumberMode = EngineSetup("renumber")
	ResetMode    = EngineSetup("reset")
)

// Convert EngineMode to String
func (m EngineMode) String() string {
	return string(m)
}

// PodmanConfig combines the defaults and settings from the file system with the
// flags given in os.Args. Some runtime state is also stored here.
type PodmanConfig struct {
	*pflag.FlagSet

	ContainersConf           *config.Config
	ContainersConfDefaultsRO *config.Config // The read-only! defaults from containers.conf.
	DBBackend                string         // Hidden: change the database backend
	DockerConfig             string         // Used for Docker compatibility
	CgroupUsage              string         // rootless code determines Usage message
	ConmonPath               string         // --conmon flag will set Engine.ConmonPath
	CPUProfile               string         // Hidden: Should CPU profile be taken
	EngineMode               EngineMode     // ABI or Tunneling mode
	Identity                 string         // ssh identity for connecting to server
	MaxWorks                 int            // maximum number of parallel threads
	MemoryProfile            string         // Hidden: Should memory profile be taken
	RegistriesConf           string         // allows for specifying a custom registries.conf
	Remote                   bool           // Connection to Podman API Service will use RESTful API
	RuntimePath              string         // --runtime flag will set Engine.RuntimePath
	RuntimeFlags             []string       // global flags for the container runtime
	Syslog                   bool           // write to StdOut and Syslog, not supported when tunneling
	Trace                    bool           // Hidden: Trace execution
	URI                      string         // URI to RESTful API Service

	Runroot        string
	ImageStore     string
	StorageDriver  string
	StorageOpts    []string
	SSHMode        string
	MachineMode    bool
	TransientStore bool
}
