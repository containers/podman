// Package for processing environment variables.
package env

// TODO: we need to add tests for this package.

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const whiteSpaces = " \t"

// DefaultEnvVariables returns a default environment, with $PATH and $TERM set.
func DefaultEnvVariables() map[string]string {
	return map[string]string{
		"PATH":      "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"container": "podman",
	}
}

// Slice transforms the specified map of environment variables into a
// slice. If a value is non-empty, the key and value are joined with '='.
func Slice(m map[string]string) []string {
	env := make([]string, 0, len(m))
	for k, v := range m {
		var s string
		if len(v) > 0 {
			s = fmt.Sprintf("%s=%s", k, v)
		} else {
			s = k
		}
		env = append(env, s)
	}
	return env
}

// Map transforms the specified slice of environment variables into a
// map.
func Map(slice []string) map[string]string {
	envmap := make(map[string]string, len(slice))
	for _, val := range slice {
		data := strings.SplitN(val, "=", 2)

		if len(data) > 1 {
			envmap[data[0]] = data[1]
		} else {
			envmap[data[0]] = ""
		}
	}
	return envmap
}

// Join joins the two environment maps with override overriding base.
func Join(base map[string]string, override map[string]string) map[string]string {
	if len(base) == 0 {
		return override
	}
	for k, v := range override {
		base[k] = v
	}
	return base
}

// ParseFile parses the specified path for environment variables and returns them
// as a map.
func ParseFile(path string) (_ map[string]string, err error) {
	env := make(map[string]string)
	defer func() {
		if err != nil {
			err = fmt.Errorf("parsing file %q: %w", path, err)
		}
	}()

	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	scanner := bufio.NewScanner(fh)
	for scanner.Scan() {
		// trim the line from all leading whitespace first
		line := strings.TrimLeft(scanner.Text(), whiteSpaces)
		// line is not empty, and not starting with '#'
		if len(line) > 0 && !strings.HasPrefix(line, "#") {
			if err := parseEnv(env, line); err != nil {
				return nil, err
			}
		}
	}
	return env, scanner.Err()
}

func parseEnv(env map[string]string, line string) error {
	data := strings.SplitN(line, "=", 2)

	// catch invalid variables such as "=" or "=A"
	if data[0] == "" {
		return fmt.Errorf("invalid variable: %q", line)
	}
	// trim the front of a variable, but nothing else
	name := strings.TrimLeft(data[0], whiteSpaces)
	if len(data) > 1 {
		env[name] = data[1]
	} else {
		if strings.HasSuffix(name, "*") {
			name = strings.TrimSuffix(name, "*")
			for _, e := range os.Environ() {
				part := strings.SplitN(e, "=", 2)
				if len(part) < 2 {
					continue
				}
				if strings.HasPrefix(part[0], name) {
					env[part[0]] = part[1]
				}
			}
		} else if val, ok := os.LookupEnv(name); ok {
			// if only a pass-through variable is given, clean it up.
			env[name] = val
		}
	}
	return nil
}
