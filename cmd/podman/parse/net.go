// most of these validate and parse functions have been taken from projectatomic/docker
// and modified for cri-o
package parse

import (
	"bufio"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/containers/common/libnetwork/etchosts"
	"github.com/containers/storage/pkg/regexp"
)

const (
	LabelType string = "label"
	ENVType   string = "env"
)

// Note: for flags that are in the form <number><unit>, use the RAMInBytes function
// from the units package in docker/go-units/size.go

var (
	whiteSpaces  = " \t"
	alphaRegexp  = regexp.Delayed(`[a-zA-Z]`)
	domainRegexp = regexp.Delayed(`^(:?(:?[a-zA-Z0-9]|(:?[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9]))(:?\.(:?[a-zA-Z0-9]|(:?[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])))*)\.?\s*$`)
)

// validateExtraHost validates that the specified string is a valid extrahost and returns it.
// ExtraHost is in the form of name:ip where the ip has to be a valid ip (ipv4 or ipv6) or the special string HostGateway.
// for add-host flag
func ValidateExtraHost(val string) (string, error) {
	// allow for IPv6 addresses in extra hosts by only splitting on first ":"
	arr := strings.SplitN(val, ":", 2)
	if len(arr) != 2 || len(arr[0]) == 0 {
		return "", fmt.Errorf("bad format for add-host: %q", val)
	}
	if arr[1] == etchosts.HostGateway {
		return val, nil
	}
	if _, err := validateIPAddress(arr[1]); err != nil {
		return "", fmt.Errorf("invalid IP address in add-host: %q", arr[1])
	}
	return val, nil
}

// validateIPAddress validates an Ip address.
// for dns, ip, and ip6 flags also
func validateIPAddress(val string) (string, error) {
	var ip = net.ParseIP(strings.TrimSpace(val))
	if ip != nil {
		return ip.String(), nil
	}
	return "", fmt.Errorf("%s is not an ip address", val)
}

func ValidateDomain(val string) (string, error) {
	if alphaRegexp.FindString(val) == "" {
		return "", fmt.Errorf("%s is not a valid domain", val)
	}
	ns := domainRegexp.FindSubmatch([]byte(val))
	if len(ns) > 0 && len(ns[1]) < 255 {
		return string(ns[1]), nil
	}
	return "", fmt.Errorf("%s is not a valid domain", val)
}

// GetAllLabels retrieves all labels given a potential label file and a number
// of labels provided from the command line.
func GetAllLabels(labelFile, inputLabels []string) (map[string]string, error) {
	labels := make(map[string]string)
	for _, file := range labelFile {
		// Use of parseEnvFile still seems safe, as it's missing the
		// extra parsing logic of parseEnv.
		// There's an argument that we SHOULD be doing that parsing for
		// all environment variables, even those sourced from files, but
		// that would require a substantial rework.
		if err := parseEnvOrLabelFile(labels, file, LabelType); err != nil {
			return nil, err
		}
	}
	for _, label := range inputLabels {
		split := strings.SplitN(label, "=", 2)
		if split[0] == "" {
			return nil, fmt.Errorf("invalid label format: %q", label)
		}
		value := ""
		if len(split) > 1 {
			value = split[1]
		}
		labels[split[0]] = value
	}
	return labels, nil
}

func parseEnvOrLabel(env map[string]string, line, configType string) error {
	data := strings.SplitN(line, "=", 2)

	// catch invalid variables such as "=" or "=A"
	if data[0] == "" {
		return fmt.Errorf("invalid environment variable: %q", line)
	}

	// trim the front of a variable, but nothing else
	name := strings.TrimLeft(data[0], whiteSpaces)
	if strings.ContainsAny(name, whiteSpaces) {
		return fmt.Errorf("name %q has white spaces, poorly formatted name", name)
	}

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
		} else if configType == ENVType {
			// if only a pass-through variable is given, clean it up.
			if val, ok := os.LookupEnv(name); ok {
				env[name] = val
			}
		}
	}
	return nil
}

// parseEnvOrLabelFile reads a file with environment variables enumerated by lines
// configType should be set to either "label" or "env" based on what type is being parsed
func parseEnvOrLabelFile(envOrLabel map[string]string, filename, configType string) error {
	fh, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer fh.Close()

	scanner := bufio.NewScanner(fh)
	for scanner.Scan() {
		// trim the line from all leading whitespace first
		line := strings.TrimLeft(scanner.Text(), whiteSpaces)
		// line is not empty, and not starting with '#'
		if len(line) > 0 && !strings.HasPrefix(line, "#") {
			if err := parseEnvOrLabel(envOrLabel, line, configType); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

// ValidURL checks a string urlStr is a url or not
func ValidURL(urlStr string) error {
	url, err := url.ParseRequestURI(urlStr)
	if err != nil {
		return fmt.Errorf("invalid url %q: %w", urlStr, err)
	}
	if url.Scheme == "" {
		return fmt.Errorf("invalid url %q: missing scheme", urlStr)
	}
	return nil
}
