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
	DockerConfig             string         // Location of authentication config file
	CgroupUsage              string         // rootless code determines Usage message
	ConmonPath               string         // --conmon flag will set Engine.ConmonPath
	CPUProfile               string         // Hidden: Should CPU profile be taken
	EngineMode               EngineMode     // ABI or Tunneling mode
	HooksDir                 []string
	Identity                 string   // ssh identity for connecting to server
	IsRenumber               bool     // Is this a system renumber command? If so, a number of checks will be relaxed
	IsReset                  bool     // Is this a system reset command? If so, a number of checks will be skipped/omitted
	MaxWorks                 int      // maximum number of parallel threads
	MemoryProfile            string   // Hidden: Should memory profile be taken
	RegistriesConf           string   // allows for specifying a custom registries.conf
	Remote                   bool     // Connection to Podman API Service will use RESTful API
	RuntimePath              string   // --runtime flag will set Engine.RuntimePath
	RuntimeFlags             []string // global flags for the container runtime
	Syslog                   bool     // write logging information to syslog as well as the console
	Trace                    bool     // Hidden: Trace execution
	URI                      string   // URI to RESTful API Service
	FarmNodeName             string   // Name of farm node
	ConnectionError          error    // Error when looking up the connection in setupRemoteConnection()

	Runroot        string
	ImageStore     string
	StorageDriver  string
	StorageOpts    []string
	SSHMode        string
	MachineMode    bool
	TransientStore bool
	GraphRoot      string
	PullOptions    []string
}
