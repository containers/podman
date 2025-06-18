//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package machine

import (
	"context"
	"errors"
	"fmt"
	"net"
)

func DialNamedPipe(ctx context.Context, path string) (net.Conn, error) {
	return nil, errors.New("not implemented")
}

func GetEnvSetString(env string, val string) string {
	return fmt.Sprintf("export %s='%s'", env, val)
}
