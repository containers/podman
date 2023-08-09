package env

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/containers/storage/pkg/regexp"
)

var (
	// Form: https://github.com/motdotla/dotenv/blob/aa03dcad1002027390dac1e8d96ac236274de354/lib/main.js#L9C76-L9C76
	// (?:export\s+)?([\w.-]+) match key
	// ([\w.%-]+)(\s*[=|*]\s*?|:\s+?) match separator
	// Remaining match value
	// e.g. KEY=VALUE => KEY, =, VALUE
	//
	//	KEY= => KEY, =, ""
	//	KEY* => KEY, *, ""
	//	KEY*=1 => KEY, *, =1
	lineRegexp = regexp.Delayed(
		`(?m)(?:^|^)\s*(?:export\s+)?([\w.%-]+)(\s*[=|*]\s*?|:\s+?)(\s*'(?:\\'|[^'])*'|\s*"(?:\\"|[^"])*"|\s*` +
			"`(?:\\`|[^`])*`" + `|[^#\r\n]+)?\s*(?:#.*)?(?:$|$)`,
	)
	onlyKeyRegexp = regexp.Delayed(`^[\w.-]+$`)
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

	content, err := io.ReadAll(fh)
	if err != nil {
		return nil, err
	}

	// replace all \r\n and \r with \n
	text := strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(string(content))
	if err := parseEnv(env, text); err != nil {
		return nil, err
	}

	return env, nil
}

// parseEnv parse the given content into env format
//
// @example: parseEnv(env, "#comment") => nil
// @example: parseEnv(env, "") => nil
// @example: parseEnv(env, "KEY=FOO") => nil
// @example: parseEnv(env, "KEY") => nil
func parseEnv(env map[string]string, content string) error {
	m := envMatch(content)

	for _, match := range m {
		key := match[1]
		separator := strings.Trim(match[2], whiteSpaces)
		value := match[3]

		if strings.Contains(value, "\n") {
			if strings.HasPrefix(value, "`") {
				return fmt.Errorf("only support multi-line environment variables surrounded by "+
					"double quotation marks or single quotation marks. invalid variable: %q", match[0])
			}

			// In the case of multi-line values, we need to remove the surrounding " '
			value = strings.Trim(value, "\"'")
		}

		// KEY*=1 => KEY, *, =1 => KEY*, =, 1
		if separator == "*" && strings.HasPrefix(value, "=") {
			key += "*"
			separator = "="
			value = strings.TrimPrefix(value, "=")
		}

		switch separator {
		case "=":
			// KEY=
			if value == "" {
				if val, ok := os.LookupEnv(key); ok {
					env[key] = val
				}
			} else {
				env[key] = value
			}
		case "*":
			for _, e := range os.Environ() {
				part := strings.SplitN(e, "=", 2)
				if len(part) < 2 {
					continue
				}
				if strings.HasPrefix(part[0], key) {
					env[part[0]] = part[1]
				}
			}
		}
	}
	return nil
}

func envMatch(content string) [][]string {
	m := lineRegexp.FindAllStringSubmatch(content, -1)

	// KEY => KEY, =, ""
	// Due to the above regex pattern, it will skip cases where only KEY is present (e.g., foo).
	// However, in our requirement, this situation is equivalent to foo=(i.e., "foo" == "foo=").
	// Therefore, we need to perform additional processing.
	// The reason for needing to support this scenario is that we need to consider: `podman run -e CI -e USERNAME`.
	{
		noMatched := lineRegexp.ReplaceAllString(content, "")
		nl := strings.Split(noMatched, "\n")
		for _, key := range nl {
			key := strings.Trim(key, whiteSpaces)
			if key == "" {
				continue
			}
			if onlyKeyRegexp.MatchString(key) {
				m = append(m, []string{key, key, "=", ""})
			}
		}
	}

	return m
}

// parseEnvWithSlice parsing a set of Env variables from a slice of strings
// because the majority of shell interpreters discard double quotes and single quotes,
// for example: podman run -e K='V', when passed into a program, it will become: K=V.
// This can lead to unexpected issues, as discussed in this link: https://github.com/containers/podman/pull/19096#issuecomment-1670164724.
//
// parseEnv method will discard all comments (#) that are not wrapped in quotation marks,
// so it cannot be used to parse env variables obtained from the command line.
//
// @example: parseEnvWithSlice(env, "KEY=FOO") => KEY: FOO
// @example: parseEnvWithSlice(env, "KEY") => KEY: ""
// @example: parseEnvWithSlice(env, "KEY=") => KEY: ""
// @example: parseEnvWithSlice(env, "KEY=FOO=BAR") => KEY: FOO=BAR
// @example: parseEnvWithSlice(env, "KEY=FOO#BAR") => KEY: FOO#BAR
func parseEnvWithSlice(env map[string]string, content string) error {
	data := strings.SplitN(content, "=", 2)

	// catch invalid variables such as "=" or "=A"
	if data[0] == "" {
		return fmt.Errorf("invalid variable: %q", content)
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
