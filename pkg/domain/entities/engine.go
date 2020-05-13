package entities

import (
	"context"
	"io"

	"github.com/containers/common/pkg/config"
	"github.com/opentracing/opentracing-go"
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
	*config.Config
	*pflag.FlagSet

	CGroupUsage    string           // rootless code determines Usage message
	ConmonPath     string           // --conmon flag will set Engine.ConmonPath
	CpuProfile     string           // Hidden: Should CPU profile be taken
	EngineMode     EngineMode       // ABI or Tunneling mode
	Identities     []string         // ssh identities for connecting to server
	MaxWorks       int              // maximum number of parallel threads
	RegistriesConf string           // allows for specifying a custom registries.conf
	RuntimePath    string           // --runtime flag will set Engine.RuntimePath
	SpanCloser     io.Closer        // Close() for tracing object
	SpanCtx        context.Context  // context to use when tracing
	Span           opentracing.Span // tracing object
	Syslog         bool             // write to StdOut and Syslog, not supported when tunneling
	Trace          bool             // Hidden: Trace execution
	Uri            string           // URI to API Service

	Runroot       string
	StorageDriver string
	StorageOpts   []string
}
