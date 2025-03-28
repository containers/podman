//go:build darwin

package apple

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"syscall"
)

// This code is adapted from github.com/crc-org/vfkit/pkg/rest/rest.go as of vkit v0.6.0.
// We don’t want to import that directly because it imports an enormous dependency tree.

// see `man unix`:
// UNIX-domain addresses are variable-length filesystem pathnames of at most 104 characters.
func maxSocketPathLen() int {
	var sockaddr syscall.RawSockaddrUnix
	// sockaddr.Path must end with '\0', it's not relevant for go strings
	return len(sockaddr.Path) - 1
}

// This is intended to be equivalent to github.com/crc-org/vfkit/pkg/rest.NewEndpoint(input).ToCmdLine()
func restNewEndpointToCmdLine(input string) ([]string, error) {
	uri, err := url.ParseRequestURI(input)
	if err != nil {
		return nil, err
	}

	switch strings.ToUpper(uri.Scheme) {
	case "NONE":
		return []string{}, nil
	case "UNIX":
		if len(uri.Path) < 1 {
			return nil, errors.New("invalid unix uri: missing path")
		}
		if len(uri.Host) > 0 {
			return nil, errors.New("invalid unix uri: host is forbidden")
		}
		if len(uri.Path) > maxSocketPathLen() {
			return nil, fmt.Errorf("invalid unix uri: socket path length exceeds macOS limits")
		}
		return []string{"--restful-uri", fmt.Sprintf("unix://%s", uri.Path)}, nil
	case "TCP", "HTTP":
		if len(uri.Host) < 1 {
			return nil, errors.New("invalid TCP uri: missing host")
		}
		if len(uri.Path) > 0 {
			return nil, errors.New("invalid TCP uri: path is forbidden")
		}
		if uri.Port() == "" {
			return nil, errors.New("invalid TCP uri: missing port")
		}
		return []string{"--restful-uri", fmt.Sprintf("tcp://%s%s", uri.Host, uri.Path)}, nil
	default:
		return nil, fmt.Errorf("invalid scheme %s", uri.Scheme)
	}
}
