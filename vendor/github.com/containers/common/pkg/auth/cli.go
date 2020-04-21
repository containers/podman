package auth

import (
	"io"

	"github.com/spf13/pflag"
)

// LoginOptions represents common flags in login
// caller should define bool or optionalBool fields for flags --get-login and --tls-verify
type LoginOptions struct {
	AuthFile      string
	CertDir       string
	GetLoginSet   bool
	Password      string
	Username      string
	StdinPassword bool
	Stdin         io.Reader
	Stdout        io.Writer
}

// LogoutOptions represents the results for flags in logout
type LogoutOptions struct {
	AuthFile string
	All      bool
	Stdin    io.Reader
	Stdout   io.Writer
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
