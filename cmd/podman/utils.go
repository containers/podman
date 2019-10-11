package main

import (
	"fmt"
	"os"
	"reflect"
	"runtime/debug"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

// print results from CLI command
func printCmdResults(ok []string, failures map[string]error) error {
	for _, id := range ok {
		fmt.Println(id)
	}

	if len(failures) > 0 {
		keys := reflect.ValueOf(failures).MapKeys()
		lastKey := keys[len(keys)-1].String()
		lastErr := failures[lastKey]
		delete(failures, lastKey)

		for _, err := range failures {
			outputError(err)
		}
		return lastErr
	}
	return nil
}

// markFlagHiddenForRemoteClient makes the flag not appear as part of the CLI
// on the remote-client
func markFlagHiddenForRemoteClient(flagName string, flags *pflag.FlagSet) {
	if remoteclient {
		if err := flags.MarkHidden(flagName); err != nil {
			debug.PrintStack()
			logrus.Errorf("unable to mark %s as hidden in the remote-client", flagName)
		}
	}
}

// markFlagHidden is a helper function to log an error if marking
// a flag as hidden happens to fail
func markFlagHidden(flags *pflag.FlagSet, flag string) {
	if err := flags.MarkHidden(flag); err != nil {
		logrus.Errorf("unable to mark flag '%s' as hidden: %q", flag, err)
	}
}

func aliasFlags(f *pflag.FlagSet, name string) pflag.NormalizedName {
	switch name {
	case "healthcheck-command":
		name = "health-cmd"
	case "healthcheck-interval":
		name = "health-interval"
	case "healthcheck-retries":
		name = "health-retries"
	case "healthcheck-start-period":
		name = "health-start-period"
	case "healthcheck-timeout":
		name = "health-timeout"
	}
	return pflag.NormalizedName(name)
}

// Check if a file exists and is not a directory
func checkIfFileExists(name string) bool {
	file, err := os.Stat(name)
	if os.IsNotExist(err) {
		return false
	}
	return !file.IsDir()
}
