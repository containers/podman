package auth

import (
	"io"

	"github.com/spf13/pflag"
)

// LoginOptions represents common flags in login
// caller should define bool or optionalBool fields for flags --get-login and --tls-verify
type LoginOptions struct {
	// CLI flags managed by the FlagSet returned by GetLoginFlags
	AuthFile      string
	CertDir       string
	Password      string
	Username      string
	StdinPassword bool
	// Options caller can set
	GetLoginSet               bool      // set to true if --get-login is explicitly set
	Stdin                     io.Reader // set to os.Stdin
	Stdout                    io.Writer // set to os.Stdout
	AcceptUnspecifiedRegistry bool      // set to true if allows login with unspecified registry
}

// LogoutOptions represents the results for flags in logout
type LogoutOptions struct {
	// CLI flags managed by the FlagSet returned by GetLogoutFlags
	AuthFile string
	All      bool
	// Options caller can set
	Stdin                     io.Reader // set to os.Stdin
	Stdout                    io.Writer // set to os.Stdout
	AcceptUnspecifiedRegistry bool      // set to true if allows logout with unspecified registry
}

// GetLoginFlags defines and returns login flags for containers tools
func GetLoginFlags(flags *LoginOptions) *pflag.FlagSet {
	fs := pflag.FlagSet{}
	fs.StringVar(&flags.AuthFile, "authfile", GetDefaultAuthFile(), "path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
	fs.StringVar(&flags.CertDir, "cert-dir", "", "use certificates at the specified path to access the registry")
	fs.StringVarP(&flags.Password, "password", "p", "", "Password for registry")
	fs.StringVarP(&flags.Username, "username", "u", "", "Username for registry")
	fs.BoolVar(&flags.StdinPassword, "password-stdin", false, "Take the password from stdin")
	return &fs
}

// GetLogoutFlags defines and returns logout flags for containers tools
func GetLogoutFlags(flags *LogoutOptions) *pflag.FlagSet {
	fs := pflag.FlagSet{}
	fs.StringVar(&flags.AuthFile, "authfile", GetDefaultAuthFile(), "path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
	fs.BoolVarP(&flags.All, "all", "a", false, "Remove the cached credentials for all registries in the auth file")
	return &fs
}
