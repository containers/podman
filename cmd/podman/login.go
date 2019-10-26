package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	buildahcli "github.com/containers/buildah/pkg/cli"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/pkg/docker/config"
	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod/image"
	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	loginCommand cliconfig.LoginValues

	loginDescription = "Login to a container registry on a specified server."
	_loginCommand    = &cobra.Command{
		Use:   "login [flags] REGISTRY",
		Short: "Login to a container registry",
		Long:  loginDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			loginCommand.InputArgs = args
			loginCommand.GlobalFlags = MainGlobalOpts
			loginCommand.Remote = remoteclient
			return loginCmd(&loginCommand)
		},
		Example: `podman login -u testuser -p testpassword localhost:5000
  podman login -u testuser -p testpassword localhost:5000`,
	}
)

func init() {
	if !remote {
		_loginCommand.Example = fmt.Sprintf("%s\n  podman login --authfile authdir/myauths.json quay.io", _loginCommand.Example)

	}
	loginCommand.Command = _loginCommand
	loginCommand.SetHelpTemplate(HelpTemplate())
	loginCommand.SetUsageTemplate(UsageTemplate())
	flags := loginCommand.Flags()

	flags.BoolVar(&loginCommand.GetLogin, "get-login", true, "Return the current login user for the registry")
	flags.StringVarP(&loginCommand.Password, "password", "p", "", "Password for registry")
	flags.StringVarP(&loginCommand.Username, "username", "u", "", "Username for registry")
	flags.BoolVar(&loginCommand.StdinPassword, "password-stdin", false, "Take the password from stdin")
	// Disabled flags for the remote client
	if !remote {
		flags.StringVar(&loginCommand.Authfile, "authfile", buildahcli.GetDefaultAuthFile(), "Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
		flags.StringVar(&loginCommand.CertDir, "cert-dir", "", "Pathname of a directory containing TLS certificates and keys used to connect to the registry")
		flags.BoolVar(&loginCommand.TlsVerify, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries")
	}
}

// loginCmd uses the authentication package to store a user's authenticated credentials
// in an auth.json file for future use
func loginCmd(c *cliconfig.LoginValues) error {
	args := c.InputArgs
	if len(args) > 1 {
		return errors.Errorf("too many arguments, login takes only 1 argument")
	}
	if len(args) == 0 {
		return errors.Errorf("please specify a registry to login to")
	}
	server := registryFromFullName(scrubServer(args[0]))

	sc := image.GetSystemContext("", c.Authfile, false)
	if c.Flag("tls-verify").Changed {
		sc.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!c.TlsVerify)
	}
	if c.CertDir != "" {
		sc.DockerCertPath = c.CertDir
	}

	// username of user logged in to server (if one exists)
	userFromAuthFile, passFromAuthFile, err := config.GetAuthentication(sc, server)
	// Do not return error if no credentials found in credHelpers, new credentials will be stored by config.SetAuthentication
	if err != nil && err != credentials.NewErrCredentialsNotFound() {
		return errors.Wrapf(err, "error reading auth file")
	}

	if c.Flag("get-login").Changed {
		if userFromAuthFile == "" {
			return errors.Errorf("not logged into %s", server)
		}
		fmt.Printf("%s\n", userFromAuthFile)
		return nil
	}

	ctx := getContext()

	password := c.Password

	if c.Flag("password-stdin").Changed {
		var stdinPasswordStrBuilder strings.Builder
		if c.Password != "" {
			return errors.Errorf("Can't specify both --password-stdin and --password")
		}
		if c.Username == "" {
			return errors.Errorf("Must provide --username with --password-stdin")
		}
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			fmt.Fprint(&stdinPasswordStrBuilder, scanner.Text())
		}
		password = stdinPasswordStrBuilder.String()
	}

	// If no username and no password is specified, try to use existing ones.
	if c.Username == "" && password == "" && userFromAuthFile != "" && passFromAuthFile != "" {
		fmt.Println("Authenticating with existing credentials...")
		if err := docker.CheckAuth(ctx, sc, userFromAuthFile, passFromAuthFile, server); err == nil {
			fmt.Println("Existing credentials are valid. Already logged in to", server)
			return nil
		}
		fmt.Println("Existing credentials are invalid, please enter valid username and password")
	}

	username, password, err := getUserAndPass(c.Username, password, userFromAuthFile)
	if err != nil {
		return errors.Wrapf(err, "error getting username and password")
	}

	if err = docker.CheckAuth(ctx, sc, username, password, server); err == nil {
		// Write the new credentials to the authfile
		if err = config.SetAuthentication(sc, server, username, password); err != nil {
			return err
		}
	}
	if err == nil {
		fmt.Println("Login Succeeded!")
		return nil
	}
	if unauthorizedError, ok := err.(docker.ErrUnauthorizedForCredentials); ok {
		logrus.Debugf("error logging into %q: %v", server, unauthorizedError)
		return errors.Errorf("error logging into %q: invalid username/password", server)
	}
	return errors.Wrapf(err, "error authenticating creds for %q", server)
}

// getUserAndPass gets the username and password from STDIN if not given
// using the -u and -p flags.  If the username prompt is left empty, the
// displayed userFromAuthFile will be used instead.
func getUserAndPass(username, password, userFromAuthFile string) (string, string, error) {
	var err error
	reader := bufio.NewReader(os.Stdin)
	if username == "" {
		if userFromAuthFile != "" {
			fmt.Printf("Username (%s): ", userFromAuthFile)
		} else {
			fmt.Print("Username: ")
		}
		username, err = reader.ReadString('\n')
		if err != nil {
			return "", "", errors.Wrapf(err, "error reading username")
		}
		// If the user just hit enter, use the displayed user from the
		// the authentication file.  This allows to do a lazy
		// `$ podman login -p $NEW_PASSWORD` without specifying the
		// user.
		if strings.TrimSpace(username) == "" {
			username = userFromAuthFile
		}
	}
	if password == "" {
		fmt.Print("Password: ")
		pass, err := terminal.ReadPassword(0)
		if err != nil {
			return "", "", errors.Wrapf(err, "error reading password")
		}
		password = string(pass)
		fmt.Println()
	}
	return strings.TrimSpace(username), password, err
}

// registryFromFullName gets the registry from the input. If the input is of the form
// quay.io/myuser/myimage, it will parse it and just return quay.io
// It also returns true if a full image name was given
func registryFromFullName(input string) string {
	split := strings.Split(input, "/")
	if len(split) > 1 {
		return split[0]
	}
	return split[0]
}
