//go:build darwin
// +build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:    "uninstall",
	Short:  "uninstalls the podman helper agent",
	Long:   "uninstalls the podman helper agent, which manages the /var/run/docker.sock link",
	PreRun: silentUsage,
	RunE:   uninstall,
}

func init() {
	addPrefixFlag(uninstallCmd)
	rootCmd.AddCommand(uninstallCmd)
}

func uninstall(cmd *cobra.Command, args []string) error {
	userName, _, _, err := getUser()
	if err != nil {
		return err
	}

	labelName := fmt.Sprintf("com.github.containers.podman.helper-%s", userName)
	fileName := filepath.Join("/Library", "LaunchDaemons", labelName+".plist")

	if err = runDetectErr("launchctl", "unload", fileName); err != nil {
		// Try removing the service by label in case the service is half uninstalled
		if rerr := runDetectErr("launchctl", "remove", labelName); rerr != nil {
			// Exit code 3 = no service to remove
			if exitErr, ok := rerr.(*exec.ExitError); !ok || exitErr.ExitCode() != 3 {
				fmt.Fprintf(os.Stderr, "Warning: service unloading failed: %s\n", err.Error())
				fmt.Fprintf(os.Stderr, "Warning: remove also failed: %s\n", rerr.Error())
			}
		}
	}

	if err := os.Remove(fileName); err != nil {
		if !os.IsNotExist(err) {
			return errors.Errorf("could not remove plist file: %s", fileName)
		}
	}

	helperPath := filepath.Join(installPrefix, "podman", "helper", userName)
	if err := os.RemoveAll(helperPath); err != nil {
		return errors.Errorf("could not remove helper binary path: %s", helperPath)
	}
	return nil
}
