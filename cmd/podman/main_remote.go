// +build remoteclient

package main

import (
	"os"
	"os/user"
	"strconv"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const remote = true

func init() {
	var username string
	if username = os.Getenv("PODMAN_USER"); username == "" {
		if curruser, err := user.Current(); err == nil {
			username = curruser.Username
		}
	}
	host := os.Getenv("PODMAN_HOST")
	port := 22
	if portstr := os.Getenv("PODMAN_PORT"); portstr != "" {
		if p, err := strconv.Atoi(portstr); err == nil {
			port = p
		}
	}
	key := os.Getenv("PODMAN_IDENTITY_FILE")
	ignore := false
	if ignorestr := os.Getenv("PODMAN_IGNORE_HOSTS"); ignorestr != "" {
		if b, err := strconv.ParseBool(ignorestr); err == nil {
			ignore = b
		}
	}
	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.ConnectionName, "connection", "", "remote connection name")
	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.RemoteConfigFilePath, "remote-config-path", "", "alternate path for configuration file")
	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.RemoteUserName, "username", username, "username on the remote host")
	rootCmd.PersistentFlags().IntVar(&MainGlobalOpts.Port, "port", port, "port on remote host")
	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.RemoteHost, "remote-host", host, "remote host")
	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.IdentityFile, "identity-file", key, "identity-file")
	rootCmd.PersistentFlags().BoolVar(&MainGlobalOpts.IgnoreHosts, "ignore-hosts", ignore, "ignore hosts")
	// TODO maybe we allow the altering of this for bridge connections?
	// rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.VarlinkAddress, "varlink-address", adapter.DefaultAddress, "address of the varlink socket")
	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.LogLevel, "log-level", "error", "Log messages above specified level: debug, info, warn, error, fatal or panic. Logged to ~/.config/containers/podman.log")
	rootCmd.PersistentFlags().BoolVar(&MainGlobalOpts.Syslog, "syslog", false, "Output logging information to syslog as well as the console")
}

func profileOn(cmd *cobra.Command) error {
	return nil
}

func profileOff(cmd *cobra.Command) error {
	return nil
}

func setupRootless(cmd *cobra.Command, args []string) error {
	return nil
}

func setRLimits() error {
	return nil
}

func setUMask() {}

// checkInput can be used to verify any of the globalopt values
func checkInput() error {
	if MainGlobalOpts.Port < 0 || MainGlobalOpts.Port > 65536 {
		return errors.Errorf("remote port must be between 0 and 65536")
	}
	return nil
}
