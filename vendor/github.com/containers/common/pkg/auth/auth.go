package auth

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/pkg/docker/config"
	"github.com/containers/image/v5/pkg/sysregistriesv2"
	"github.com/containers/image/v5/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	terminal "golang.org/x/term"
)

// GetDefaultAuthFile returns env value REGISTRY_AUTH_FILE as default
// --authfile path used in multiple --authfile flag definitions
// Will fail over to DOCKER_CONFIG if REGISTRY_AUTH_FILE environment is not set
func GetDefaultAuthFile() string {
	if authfile := os.Getenv("REGISTRY_AUTH_FILE"); authfile != "" {
		return authfile
	}
	if auth_env := os.Getenv("DOCKER_CONFIG"); auth_env != "" {
		return filepath.Join(auth_env, "config.json")
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
		return errors.Wrap(err, "checking authfile")
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
		authConfig    types.DockerAuthConfig
		key, registry string
		ref           reference.Named
		err           error
	)
	l := len(args)
	switch l {
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
		key, registry, ref, err = parseRegistryArgument(args[0], opts.AcceptRepositories)
		if err != nil {
			return err
		}

	default:
		return errors.New("login accepts only one registry to login to")

	}

	if ref != nil {
		authConfig, err = config.GetCredentialsForRef(systemContext, ref)
		if err != nil {
			return errors.Wrap(err, "get credentials for repository")
		}
	} else {
		authConfig, err = config.GetCredentials(systemContext, registry)
		if err != nil {
			return errors.Wrap(err, "get credentials")
		}
	}

	if opts.GetLoginSet {
		if authConfig.Username == "" {
			return errors.Errorf("not logged into %s", key)
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
		return errors.Wrap(err, "getting username and password")
	}

	if err = docker.CheckAuth(ctx, systemContext, username, password, registry); err == nil {
		// Write the new credentials to the authfile
		desc, err := config.SetCredentials(systemContext, key, username, password)
		if err != nil {
			return err
		}
		if opts.Verbose {
			fmt.Fprintln(opts.Stdout, "Used: ", desc)
		}
	}
	if err == nil {
		fmt.Fprintln(opts.Stdout, "Login Succeeded!")
		return nil
	}
	if unauthorized, ok := err.(docker.ErrUnauthorizedForCredentials); ok {
		logrus.Debugf("error logging into %q: %v", key, unauthorized)
		return errors.Errorf("error logging into %q: invalid username/password", key)
	}
	return errors.Wrapf(err, "authenticating creds for %q", key)
}

// parseRegistryArgument verifies the provided arg depending if we accept
// repositories or not.
func parseRegistryArgument(arg string, acceptRepositories bool) (key, registry string, maybeRef reference.Named, err error) {
	if !acceptRepositories {
		registry = getRegistryName(arg)
		key = registry
		return key, registry, maybeRef, nil
	}

	key = trimScheme(arg)
	if key != arg {
		return key, registry, nil, errors.New("credentials key has https[s]:// prefix")
	}

	registry = getRegistryName(key)
	if registry == key {
		// We cannot parse a reference from a registry, so we stop here
		return key, registry, nil, nil
	}

	ref, parseErr := reference.ParseNamed(key)
	if parseErr != nil {
		return key, registry, nil, errors.Wrapf(parseErr, "parse reference from %q", key)
	}

	if !reference.IsNameOnly(ref) {
		return key, registry, nil, errors.Errorf("reference %q contains tag or digest", ref.String())
	}

	maybeRef = ref
	registry = reference.Domain(ref)

	return key, registry, maybeRef, nil
}

// getRegistryName scrubs and parses the input to get the server name
func getRegistryName(server string) string {
	// removes 'http://' or 'https://' from the front of the
	// server/registry string if either is there.  This will be mostly used
	// for user input from 'Buildah login' and 'Buildah logout'.
	server = trimScheme(server)
	// gets the registry from the input. If the input is of the form
	// quay.io/myuser/myimage, it will parse it and just return quay.io
	split := strings.Split(server, "/")
	return split[0]
}

// trimScheme removes the HTTP(s) scheme from the provided repository.
func trimScheme(repository string) string {
	// removes 'http://' or 'https://' from the front of the
	// server/registry string if either is there.  This will be mostly used
	// for user input from 'Buildah login' and 'Buildah logout'.
	return strings.TrimPrefix(strings.TrimPrefix(repository, "https://"), "http://")
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
			return "", "", errors.Wrap(err, "reading username")
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
		pass, err := terminal.ReadPassword(0)
		if err != nil {
			return "", "", errors.Wrap(err, "reading password")
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

	var (
		key, registry string
		ref           reference.Named
		err           error
	)
	if len(args) > 1 {
		return errors.New("logout accepts only one registry to logout from")
	}
	if len(args) == 0 && !opts.All {
		if !opts.AcceptUnspecifiedRegistry {
			return errors.New("please provide a registry to logout from")
		}
		if key, err = defaultRegistryWhenUnspecified(systemContext); err != nil {
			return err
		}
		registry = key
		logrus.Debugf("registry not specified, default to the first registry %q from registries.conf", key)
	}
	if len(args) != 0 {
		if opts.All {
			return errors.New("--all takes no arguments")
		}
		key, registry, ref, err = parseRegistryArgument(args[0], opts.AcceptRepositories)
		if err != nil {
			return err
		}
	}

	if opts.All {
		if err := config.RemoveAllAuthentication(systemContext); err != nil {
			return err
		}
		fmt.Fprintln(opts.Stdout, "Removed login credentials for all registries")
		return nil
	}

	err = config.RemoveAuthentication(systemContext, key)
	switch errors.Cause(err) {
	case nil:
		fmt.Fprintf(opts.Stdout, "Removed login credentials for %s\n", key)
		return nil
	case config.ErrNotLoggedIn:
		var authConfig types.DockerAuthConfig
		if ref != nil {
			authConfig, err = config.GetCredentialsForRef(systemContext, ref)
			if err != nil {
				return errors.Wrap(err, "get credentials for repository")
			}
		} else {
			authConfig, err = config.GetCredentials(systemContext, registry)
			if err != nil {
				return errors.Wrap(err, "get credentials")
			}
		}

		authInvalid := docker.CheckAuth(context.Background(), systemContext, authConfig.Username, authConfig.Password, registry)
		if authConfig.Username != "" && authConfig.Password != "" && authInvalid == nil {
			fmt.Printf("Not logged into %s with current tool. Existing credentials were established via docker login. Please use docker logout instead.\n", key)
			return nil
		}
		return errors.Errorf("Not logged into %s\n", key)
	default:
		return errors.Wrapf(err, "logging out of %q", key)
	}
}

// defaultRegistryWhenUnspecified returns first registry from search list of registry.conf
// used by login/logout when registry argument is not specified
func defaultRegistryWhenUnspecified(systemContext *types.SystemContext) (string, error) {
	registriesFromFile, err := sysregistriesv2.UnqualifiedSearchRegistries(systemContext)
	if err != nil {
		return "", errors.Wrap(err, "getting registry from registry.conf, please specify a registry")
	}
	if len(registriesFromFile) == 0 {
		return "", errors.New("no registries found in registries.conf, a registry must be provided")
	}
	return registriesFromFile[0], nil
}
