package imagebuilder

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// hasEnvName returns true if the provided environment contains the named ENV var.
func hasEnvName(env []string, name string) bool {
	for _, e := range env {
		if strings.HasPrefix(e, name+"=") {
			return true
		}
	}
	return false
}

// platformSupports is a short-term function to give users a quality error
// message if a Dockerfile uses a command not supported on the platform.
func platformSupports(command string) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	switch command {
	case "expose", "user", "stopsignal", "arg":
		return fmt.Errorf("The daemon on this platform does not support the command '%s'", command)
	}
	return nil
}

func handleJSONArgs(args []string, attributes map[string]bool) []string {
	if len(args) == 0 {
		return []string{}
	}

	if attributes != nil && attributes["json"] {
		return args
	}

	// literal string command, not an exec array
	return []string{strings.Join(args, " ")}
}

func hasSlash(input string) bool {
	return strings.HasSuffix(input, string(os.PathSeparator)) || strings.HasSuffix(input, string(os.PathSeparator)+".")
}

// makeAbsolute ensures that the provided path is absolute.
func makeAbsolute(dest, workingDir string) string {
	// Twiddle the destination when its a relative path - meaning, make it
	// relative to the WORKINGDIR
	if dest == "." {
		if !hasSlash(workingDir) {
			workingDir += string(os.PathSeparator)
		}
		dest = workingDir
	}

	if !filepath.IsAbs(dest) {
		hasSlash := hasSlash(dest)
		dest = filepath.Join(string(os.PathSeparator), filepath.FromSlash(workingDir), dest)

		// Make sure we preserve any trailing slash
		if hasSlash {
			dest += string(os.PathSeparator)
		}
	}
	return dest
}

// parseOptInterval(flag) is the duration of flag.Value, or 0 if
// empty. An error is reported if the value is given and is not positive.
func parseOptInterval(f *flag.Flag) (time.Duration, error) {
	if f == nil {
		return 0, fmt.Errorf("No flag defined")
	}
	s := f.Value.String()
	if s == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	if d <= 0 {
		return 0, fmt.Errorf("Interval %#v must be positive", f.Name)
	}
	return d, nil
}

// makeUserArgs - Package the variables from the Dockerfile defined by
// the ENV aand the ARG statements into one slice so the values
// defined by both can later be evaluated when resolving variables
// such as ${MY_USER}.  If the variable is defined by both ARG and ENV
// don't include the definition of the ARG variable.
func makeUserArgs(bEnv []string, bArgs map[string]string) (userArgs []string) {

	userArgs = bEnv
	envMap := make(map[string]string)
	for _, envVal := range bEnv {
		val := strings.SplitN(envVal, "=", 2)
		if len(val) > 1 {
			envMap[val[0]] = val[1]
		}
	}

	for key, value := range bArgs {
		if _, ok := envMap[key]; ok {
			continue
		}
		userArgs = append(userArgs, key+"="+value)
	}
	return userArgs
}
