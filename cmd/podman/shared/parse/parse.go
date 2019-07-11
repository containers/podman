//nolint
// most of these validate and parse functions have been taken from projectatomic/docker
// and modified for cri-o
package parse

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

const (
	Protocol_TCP Protocol = 0
	Protocol_UDP Protocol = 1
)

type Protocol int32

// PortMapping specifies the port mapping configurations of a sandbox.
type PortMapping struct {
	// Protocol of the port mapping.
	Protocol Protocol `protobuf:"varint,1,opt,name=protocol,proto3,enum=runtime.Protocol" json:"protocol,omitempty"`
	// Port number within the container. Default: 0 (not specified).
	ContainerPort int32 `protobuf:"varint,2,opt,name=container_port,json=containerPort,proto3" json:"container_port,omitempty"`
	// Port number on the host. Default: 0 (not specified).
	HostPort int32 `protobuf:"varint,3,opt,name=host_port,json=hostPort,proto3" json:"host_port,omitempty"`
	// Host IP.
	HostIp string `protobuf:"bytes,4,opt,name=host_ip,json=hostIp,proto3" json:"host_ip,omitempty"`
}

// Note: for flags that are in the form <number><unit>, use the RAMInBytes function
// from the units package in docker/go-units/size.go

var (
	whiteSpaces  = " \t"
	alphaRegexp  = regexp.MustCompile(`[a-zA-Z]`)
	domainRegexp = regexp.MustCompile(`^(:?(:?[a-zA-Z0-9]|(:?[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9]))(:?\.(:?[a-zA-Z0-9]|(:?[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])))*)\.?\s*$`)
)

// validateExtraHost validates that the specified string is a valid extrahost and returns it.
// ExtraHost is in the form of name:ip where the ip has to be a valid ip (ipv4 or ipv6).
// for add-host flag
func ValidateExtraHost(val string) (string, error) { //nolint
	// allow for IPv6 addresses in extra hosts by only splitting on first ":"
	arr := strings.SplitN(val, ":", 2)
	if len(arr) != 2 || len(arr[0]) == 0 {
		return "", fmt.Errorf("bad format for add-host: %q", val)
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

// reads a file of line terminated key=value pairs, and overrides any keys
// present in the file with additional pairs specified in the override parameter
// for env-file and labels-file flags
func ReadKVStrings(env map[string]string, files []string, override []string) error {
	for _, ef := range files {
		if err := parseEnvFile(env, ef); err != nil {
			return err
		}
	}
	for _, line := range override {
		if err := parseEnv(env, line); err != nil {
			return err
		}
	}
	return nil
}

func parseEnv(env map[string]string, line string) error {
	data := strings.SplitN(line, "=", 2)

	// catch invalid variables such as "=" or "=A"
	if data[0] == "" {
		return errors.Errorf("invalid environment variable: %q", line)
	}

	// trim the front of a variable, but nothing else
	name := strings.TrimLeft(data[0], whiteSpaces)
	if strings.ContainsAny(name, whiteSpaces) {
		return errors.Errorf("name %q has white spaces, poorly formatted name", name)
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
		} else {
			// if only a pass-through variable is given, clean it up.
			val, _ := os.LookupEnv(name)
			env[name] = val
		}
	}
	return nil
}

// parseEnvFile reads a file with environment variables enumerated by lines
func parseEnvFile(env map[string]string, filename string) error {
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
			if err := parseEnv(env, line); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

// ValidateFileName returns an error if filename contains ":"
// as it is currently not supported
func ValidateFileName(filename string) error {
	if strings.Contains(filename, ":") {
		return errors.Errorf("invalid filename (should not contain ':') %q", filename)
	}
	return nil
}
