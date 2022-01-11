package utils

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	buildahCLI "github.com/containers/buildah/pkg/cli"
)

type OutputErrors []error

func (o OutputErrors) PrintErrors() (lastError error) {
	if len(o) == 0 {
		return
	}
	lastError = o[len(o)-1]
	for e := 0; e < len(o)-1; e++ {
		fmt.Fprintf(os.Stderr, "Error: %s\n", o[e])
	}
	return
}

/* For remote client, server does not returns error with exit code
   instead returns a message and we cast it to a new error.

   Following function performs parsing on build error and returns
   exit status which was expected for this current build
*/
func ExitCodeFromBuildError(errorMsg string) (int, error) {
	if strings.Contains(errorMsg, "exit status") {
		errorSplit := strings.Split(errorMsg, " ")
		if errorSplit[len(errorSplit)-2] == "status" {
			tmpSplit := strings.Split(errorSplit[len(errorSplit)-1], "\n")
			exitCodeRemote, err := strconv.Atoi(tmpSplit[0])
			if err == nil {
				return exitCodeRemote, nil
			}
			return buildahCLI.ExecErrorCodeGeneric, err
		}
	}
	return buildahCLI.ExecErrorCodeGeneric, errors.New("error message does not contains a valid exit code")
}
