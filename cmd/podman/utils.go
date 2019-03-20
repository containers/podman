package main

import (
	"fmt"

	"github.com/spf13/pflag"
)

//printParallelOutput takes the map of parallel worker results and outputs them
// to stdout
func printParallelOutput(m map[string]error, errCount int) error {
	var lastError error
	for cid, result := range m {
		if result != nil {
			if errCount > 1 {
				fmt.Println(result.Error())
			}
			lastError = result
			continue
		}
		fmt.Println(cid)
	}
	return lastError
}

// markFlagHiddenForRemoteClient makes the flag not appear as part of the CLI
// on the remote-client
func markFlagHiddenForRemoteClient(flagName string, flags *pflag.FlagSet) {
	if remoteclient {
		flags.MarkHidden(flagName)
	}
}
