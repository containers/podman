package auth

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/pkg/docker/config"
	"github.com/containers/image/v5/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

// GetDefaultAuthFile returns env value REGISTRY_AUTH_FILE as default --authfile path
// used in multiple --authfile flag definitions
func GetDefaultAuthFile() string {
	return os.Getenv("REGISTRY_AUTH_FILE")
}

// CheckAuthFile validates filepath given by --authfile
// used by command has --authfile flag
func CheckAuthFile(authfile string) error {
	if authfile == "" {
		return nil
	}
	if _, err := os.Stat(authfile); err != nil {
		return errors.Wrapf(err, "error checking authfile path %s", authfile)
	}
	return nil
}

// Login login to the server with creds from Stdin or CLI
func Login(ctx context.Context, systemContext *types.SystemContext, opts *LoginOptions, registry string) error {
	server := getRegistryName(registry)
	authConfig, err := config.GetCredentials(systemContext, server)
	if err != nil {
		return errors.Wrapf(err, "error reading auth file")
	}
	if opts.GetLoginSet {
		if authConfig.Username == "" {
			return errors.Errorf("not logged into %s", server)
		}
		fmt.Fprintf(opts.Stdout, "%s\n", authConfig.Username)
		return nil
	}
	if authConfig.IdentityToken != "" {
		return errors.Errorf("currently logged in, auth file contains an Identity token")
	}

	password := opts.Password
	if opts.StdinPassword {
		var stdinPasswordStrBuilder strings.Builder
		if opts.Password != "" {
			return errors.Errorf("Can't specify both --password-stdin and --password")
		}
		if opts.Username == "" {
			return errors.Errorf("Must provide --username with --password-stdin")
		}
		scanner := bufio.NewScanner(opts.Stdin)
		for scanner.Scan() {
			fmt.Fprint(&stdinPasswordStrBuilder, scanner.Text())
		}
		password = stdinPasswordStrBuilder.String()
	}

	// If no username and no password is specified, try to use existing ones.
	if opts.Username == "" && password == "" && authConfig.Username != "" && authConfig.Password != "" {
		fmt.Println("Authenticating with existing credentials...")
		if err := docker.CheckAuth(ctx, systemContext, authConfig.Username, authConfig.Password, server); err == nil {
			fmt.Fprintln(opts.Stdout, "Existing credentials are valid. Already logged in to", server)
			return nil
		}
		fmt.Fprintln(opts.Stdout, "Existing credentials are invalid, please enter valid username and password")
	}

	username, password, err := getUserAndPass(opts, password, authConfig.Username)
	if err != nil {
		return errors.Wrapf(err, "error getting username and password")
	}

	if err = docker.CheckAuth(ctx, systemContext, username, password, server); err == nil {
		// Write the new credentials to the authfile
		if err = config.SetAuthentication(systemContext, server, username, password); err != nil {
			return err
		}
	}
	if err == nil {
		fmt.Fprintln(opts.Stdout, "Login Succeeded!")
		return nil
	}
	if unauthorized, ok := err.(docker.ErrUnauthorizedForCredentials); ok {
		logrus.Debugf("error logging into %q: %v", server, unauthorized)
		return errors.Errorf("error logging into %q: invalid username/password", server)
	}
	return errors.Wrapf(err, "error authenticating creds for %q", server)
}

// getRegistryName scrubs and parses the input to get the server name
func getRegistryName(server string) string {
	// removes 'http://' or 'https://' from the front of the
	// server/registry string if either is there.  This will be mostly used
	// for user input from 'Buildah login' and 'Buildah logout'.
	server = strings.TrimPrefix(strings.TrimPrefix(server, "https://"), "http://")
	// gets the registry from the input. If the input is of the form
	// quay.io/myuser/myimage, it will parse it and just return quay.io
	split := strings.Split(server, "/")
	if len(split) > 1 {
		return split[0]
	}
	return split[0]
}

// getUserAndPass gets the username and password from STDIN if not given
// using the -u and -p flags.  If the username prompt is left empty, the
// displayed userFromAuthFile will be used instead.
func getUserAndPass(opts *LoginOptions, password, userFromAuthFile string) (string, string, error) {
	var err error
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
			return "", "", errors.Wrapf(err, "error reading username")
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
			return "", "", errors.Wrapf(err, "error reading password")
		}
		password = string(pass)
		fmt.Fprintln(opts.Stdout)
	}
	return strings.TrimSpace(username), password, err
}

// Logout removes the authentication of server from authfile
// removes all authtication if specifies all in the options
func Logout(systemContext *types.SystemContext, opts *LogoutOptions, server string) error {
	if server != "" {
		server = getRegistryName(server)
	}
	if err := CheckAuthFile(opts.AuthFile); err != nil {
		return err
	}

	if opts.All {
		if err := config.RemoveAllAuthentication(systemContext); err != nil {
			return err
		}
		fmt.Fprintln(opts.Stdout, "Removed login credentials for all registries")
		return nil
	}

	err := config.RemoveAuthentication(systemContext, server)
	switch err {
	case nil:
		fmt.Fprintf(opts.Stdout, "Removed login credentials for %s\n", server)
		return nil
	case config.ErrNotLoggedIn:
		return errors.Errorf("Not logged into %s\n", server)
	default:
		return errors.Wrapf(err, "error logging out of %q", server)
	}
}
