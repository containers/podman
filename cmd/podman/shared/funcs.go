package shared

import (
	"fmt"
	"os"
	"strings"
)

// GenerateCommand takes a label (string) and converts it to an executable command
func GenerateCommand(command, imageName, name string) []string {
	var (
		newCommand []string
	)
	if name == "" {
		name = imageName
	}
	cmd := strings.Split(command, " ")
	// Replace the first element of cmd with "/proc/self/exe"
	newCommand = append(newCommand, "/proc/self/exe")
	for _, arg := range cmd[1:] {
		var newArg string
		switch arg {
		case "IMAGE":
			newArg = imageName
		case "IMAGE=IMAGE":
			newArg = fmt.Sprintf("IMAGE=%s", imageName)
		case "IMAGE=$IMAGE":
			newArg = fmt.Sprintf("IMAGE=%s", imageName)
		case "NAME":
			newArg = name
		case "NAME=NAME":
			newArg = fmt.Sprintf("NAME=%s", name)
		case "NAME=$NAME":
			newArg = fmt.Sprintf("NAME=%s", name)
		default:
			newArg = arg
		}
		newCommand = append(newCommand, newArg)
	}
	return newCommand
}

// GenerateRunEnvironment merges the current environment variables with optional
// environment variables provided by the user
func GenerateRunEnvironment(name, imageName string, opts map[string]string) []string {
	newEnv := os.Environ()
	newEnv = append(newEnv, fmt.Sprintf("NAME=%s", name))
	newEnv = append(newEnv, fmt.Sprintf("IMAGE=%s", imageName))

	if opts["opt1"] != "" {
		newEnv = append(newEnv, fmt.Sprintf("OPT1=%s", opts["opt1"]))
	}
	if opts["opt2"] != "" {
		newEnv = append(newEnv, fmt.Sprintf("OPT2=%s", opts["opt2"]))
	}
	if opts["opt3"] != "" {
		newEnv = append(newEnv, fmt.Sprintf("OPT3=%s", opts["opt3"]))
	}
	return newEnv
}
