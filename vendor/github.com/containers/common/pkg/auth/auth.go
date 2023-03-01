package auth

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/pkg/docker/config"
	"github.com/containers/image/v5/pkg/sysregistriesv2"
	"github.com/containers/image/v5/types"
	"github.com/sirupsen/logrus"
	terminal "golang.org/x/term"
)

// ErrNewCredentialsInvalid means that the new user-provided credentials are
// not accepted by the registry.
type ErrNewCredentialsInvalid struct {
	underlyingError error
	message         string
}

// Error returns the error message as a string.
func (e ErrNewCredentialsInvalid) Error() string {
	return e.message
}

// Unwrap returns the underlying error.
func (e ErrNewCredentialsInvalid) Unwrap() error {
	return e.underlyingError
}

// GetDefaultAuthFile returns env value REGISTRY_AUTH_FILE as default
// --authfile path used in multiple --authfile flag definitions
// Will fail over to DOCKER_CONFIG if REGISTRY_AUTH_FILE environment is not set
func GetDefaultAuthFile() string {
	if authfile := os.Getenv("REGISTRY_AUTH_FILE"); authfile != "" {
		return authfile
	}
	if authEnv := os.Getenv("DOCKER_CONFIG"); authEnv != "" {
		return filepath.Join(authEnv, "config.json")
	}
	return ""
}

// CheckAuthFile validates filepath given by --authfile
// used by command has --authfile flag
func CheckAuthFile(authfile string) error {
	if authfile == "" {
		return nil
	}
	if _, err := os.Stat(authfile); err != nil {
		return fmt.Errorf("checking authfile: %w", err)
	}
	return nil
}

// systemContextWithOptions returns a version of sys
// updated with authFile and certDir values (if they are not "").
// NOTE: this is a shallow copy that can be used and updated, but may share
// data with the original parameter.
func systemContextWithOptions(sys *types.SystemContext, authFile, certDir string) *types.SystemContext {
	if sys != nil {
		sysCopy := *sys
		sys = &sysCopy
	} else {
		sys = &types.SystemContext{}
	}

	if authFile != "" {
		sys.AuthFilePath = authFile
	}
	if certDir != "" {
		sys.DockerCertPath = certDir
	}
	return sys
}

// Login implements a “log in” command with the provided opts and args
// reading the password from opts.Stdin or the options in opts.
func Login(ctx context.Context, systemContext *types.SystemContext, opts *LoginOptions, args []string) error {
	systemContext = systemContextWithOptions(systemContext, opts.AuthFile, opts.CertDir)

	var (
		key, registry string
		err           error
	)
	switch len(args) {
	case 0:
		if !opts.AcceptUnspecifiedRegistry {
			return errors.New("please provide a registry to login to")
		}
		if key, err = defaultRegistryWhenUnspecified(systemContext); err != nil {
			return err
		}
		registry = key
		logrus.Debugf("registry not specified, default to the first registry %q from registries.conf", key)

	case 1:
		key, registry, err = parseCredentialsKey(args[0], opts.AcceptRepositories)
		if err != nil {
			return err
		}

	default:
		return errors.New("login accepts only one registry to login to")
	}

	authConfig, err := config.GetCredentials(systemContext, key)
	if err != nil {
		return fmt.Errorf("get credentials: %w", err)
	}

	if opts.GetLoginSet {
		if authConfig.Username == "" {
			return fmt.Errorf("not logged into %s", key)
		}
		fmt.Fprintf(opts.Stdout, "%s\n", authConfig.Username)
		return nil
	}
	if authConfig.IdentityToken != "" {
		return errors.New("currently logged in, auth file contains an Identity token")
	}

	password := opts.Password
	if opts.StdinPassword {
		var stdinPasswordStrBuilder strings.Builder
		if opts.Password != "" {
			return errors.New("Can't specify both --password-stdin and --password")
		}
		if opts.Username == "" {
			return errors.New("Must provide --username with --password-stdin")
		}
		scanner := bufio.NewScanner(opts.Stdin)
		for scanner.Scan() {
			fmt.Fprint(&stdinPasswordStrBuilder, scanner.Text())
		}
		password = stdinPasswordStrBuilder.String()
	}

	// If no username and no password is specified, try to use existing ones.
	if opts.Username == "" && password == "" && authConfig.Username != "" && authConfig.Password != "" {
		fmt.Fprintf(opts.Stdout, "Authenticating with existing credentials for %s\n", key)
		if err := docker.CheckAuth(ctx, systemContext, authConfig.Username, authConfig.Password, registry); err == nil {
			fmt.Fprintf(opts.Stdout, "Existing credentials are valid. Already logged in to %s\n", registry)
			return nil
		}
		fmt.Fprintln(opts.Stdout, "Existing credentials are invalid, please enter valid username and password")
	}

	username, password, err := getUserAndPass(opts, password, authConfig.Username)
	if err != nil {
		return fmt.Errorf("getting username and password: %w", err)
	}

	if err = docker.CheckAuth(ctx, systemContext, username, password, registry); err == nil {
		if !opts.NoWriteBack {
			// Write the new credentials to the authfile
			desc, err := config.SetCredentials(systemContext, key, username, password)
			if err != nil {
				return err
			}
			if opts.Verbose {
				fmt.Fprintln(opts.Stdout, "Used: ", desc)
			}
		}
		fmt.Fprintln(opts.Stdout, "Login Succeeded!")
		return nil
	}
	if unauthorized, ok := err.(docker.ErrUnauthorizedForCredentials); ok {
		logrus.Debugf("error logging into %q: %v", key, unauthorized)
		return ErrNewCredentialsInvalid{
			underlyingError: err,
			message:         fmt.Sprintf("logging into %q: invalid username/password", key),
		}
	}
	return fmt.Errorf("authenticating creds for %q: %w", key, err)
}

// parseCredentialsKey turns the provided argument into a valid credential key
// and computes the registry part.
func parseCredentialsKey(arg string, acceptRepositories bool) (key, registry string, err error) {
	// URL arguments are replaced with their host[:port] parts.
	key, err = replaceURLByHostPort(arg)
	if err != nil {
		return "", "", err
	}

	split := strings.Split(key, "/")
	registry = split[0]

	if !acceptRepositories {
		return registry, registry, nil
	}

	// Return early if the key isn't namespaced or uses an http{s} prefix.
	if registry == key {
		return key, registry, nil
	}

	// Sanity-check that the key looks reasonable (e.g. doesn't use invalid characters),
	// and does not contain a tag or digest.
	// WARNING: ref.Named() MUST NOT be used to compute key, because
	// reference.ParseNormalizedNamed() turns docker.io/vendor to docker.io/library/vendor
	// Ideally c/image should provide dedicated validation functionality.
	ref, err := reference.ParseNormalizedNamed(key)
	if err != nil {
		return "", "", fmt.Errorf("parse reference from %q: %w", key, err)
	}
	if !reference.IsNameOnly(ref) {
		return "", "", fmt.Errorf("reference %q contains tag or digest", ref.String())
	}
	refRegistry := reference.Domain(ref)
	if refRegistry != registry { // This should never happen, check just to make sure
		return "", "", fmt.Errorf("internal error: key %q registry mismatch, %q vs. %q", key, ref, refRegistry)
	}

	return key, registry, nil
}

// If the specified string starts with http{s} it is replaced with it's
// host[:port] parts; everything else is stripped. Otherwise, the string is
// returned as is.
func replaceURLByHostPort(repository string) (string, error) {
	if !strings.HasPrefix(repository, "https://") && !strings.HasPrefix(repository, "http://") {
		return repository, nil
	}
	u, err := url.Parse(repository)
	if err != nil {
		return "", fmt.Errorf("trimming http{s} prefix: %v", err)
	}
	return u.Host, nil
}

// getUserAndPass gets the username and password from STDIN if not given
// using the -u and -p flags.  If the username prompt is left empty, the
// displayed userFromAuthFile will be used instead.
func getUserAndPass(opts *LoginOptions, password, userFromAuthFile string) (user, pass string, err error) {
	reader := bufio.NewReader(opts.Stdin)
	username := opts.Username
	if username == "" {
		if userFromAuthFile != "" {
			fmt.Fprintf(opts.Stdout, "Username (%s): ", userFromAuthFile)
		} else {
			fmt.Fprint(opts.Stdout, "Username: ")
		}
		username, err = reader.ReadString('\n')
		if err != nil {
			return "", "", fmt.Errorf("reading username: %w", err)
		}
		// If the user just hit enter, use the displayed user from the
		// the authentication file.  This allows to do a lazy
		// `$ buildah login -p $NEW_PASSWORD` without specifying the
		// user.
		if strings.TrimSpace(username) == "" {
			username = userFromAuthFile
		}
	}
	if password == "" {
		fmt.Fprint(opts.Stdout, "Password: ")
		pass, err := terminal.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return "", "", fmt.Errorf("reading password: %w", err)
		}
		password = string(pass)
		fmt.Fprintln(opts.Stdout)
	}
	return strings.TrimSpace(username), password, err
}

// Logout implements a “log out” command with the provided opts and args
func Logout(systemContext *types.SystemContext, opts *LogoutOptions, args []string) error {
	if err := CheckAuthFile(opts.AuthFile); err != nil {
		return err
	}
	systemContext = systemContextWithOptions(systemContext, opts.AuthFile, "")

	if opts.All {
		if len(args) != 0 {
			return errors.New("--all takes no arguments")
		}
		if err := config.RemoveAllAuthentication(systemContext); err != nil {
			return err
		}
		fmt.Fprintln(opts.Stdout, "Removed login credentials for all registries")
		return nil
	}

	var (
		key, registry string
		err           error
	)
	switch len(args) {
	case 0:
		if !opts.AcceptUnspecifiedRegistry {
			return errors.New("please provide a registry to logout from")
		}
		if key, err = defaultRegistryWhenUnspecified(systemContext); err != nil {
			return err
		}
		registry = key
		logrus.Debugf("registry not specified, default to the first registry %q from registries.conf", key)

	case 1:
		key, registry, err = parseCredentialsKey(args[0], opts.AcceptRepositories)
		if err != nil {
			return err
		}

	default:
		return errors.New("logout accepts only one registry to logout from")
	}

	err = config.RemoveAuthentication(systemContext, key)
	if err == nil {
		fmt.Fprintf(opts.Stdout, "Removed login credentials for %s\n", key)
		return nil
	}

	if errors.Is(err, config.ErrNotLoggedIn) {
		authConfig, err := config.GetCredentials(systemContext, key)
		if err != nil {
			return fmt.Errorf("get credentials: %w", err)
		}

		authInvalid := docker.CheckAuth(context.Background(), systemContext, authConfig.Username, authConfig.Password, registry)
		if authConfig.Username != "" && authConfig.Password != "" && authInvalid == nil {
			fmt.Printf("Not logged into %s with current tool. Existing credentials were established via docker login. Please use docker logout instead.\n", key)
			return nil
		}
		return fmt.Errorf("not logged into %s", key)
	}

	return fmt.Errorf("logging out of %q: %w", key, err)
}

// defaultRegistryWhenUnspecified returns first registry from search list of registry.conf
// used by login/logout when registry argument is not specified
func defaultRegistryWhenUnspecified(systemContext *types.SystemContext) (string, error) {
	registriesFromFile, err := sysregistriesv2.UnqualifiedSearchRegistries(systemContext)
	if err != nil {
		return "", fmt.Errorf("getting registry from registry.conf, please specify a registry: %w", err)
	}
	if len(registriesFromFile) == 0 {
		return "", errors.New("no registries found in registries.conf, a registry must be provided")
	}
	return registriesFromFile[0], nil
}
