package main

import (
	"fmt"
	"reflect"

	"github.com/spf13/pflag"
)

// printParallelOutput takes the map of parallel worker results and outputs them
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
		flags.MarkHidden(flagName)
	}
}

// TODO: remove when adapter package takes over this functionality
// func joinContainerOrCreateRootlessUserNS(runtime *libpod.Runtime, ctr *libpod.Container) (bool, int, error) {
// 	if os.Geteuid() == 0 {
// 		return false, 0, nil
// 	}
// 	s, err := ctr.State()
// 	if err != nil {
// 		return false, -1, err
// 	}
// 	opts := rootless.Opts{
// 		Argument: ctr.ID(),
// 	}
// 	if s == libpod.ContainerStateRunning || s == libpod.ContainerStatePaused {
// 		data, err := ioutil.ReadFile(ctr.Config().ConmonPidFile)
// 		if err != nil {
// 			return false, -1, errors.Wrapf(err, "cannot read conmon PID file %q", ctr.Config().ConmonPidFile)
// 		}
// 		conmonPid, err := strconv.Atoi(string(data))
// 		if err != nil {
// 			return false, -1, errors.Wrapf(err, "cannot parse PID %q", data)
// 		}
// 		return rootless.JoinDirectUserAndMountNSWithOpts(uint(conmonPid), &opts)
// 	}
// 	return rootless.BecomeRootInUserNSWithOpts(&opts)
// }
