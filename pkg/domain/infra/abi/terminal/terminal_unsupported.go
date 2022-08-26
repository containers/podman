//go:build !linux && !freebsd
// +build !linux,!freebsd

package terminal

import (
	"context"
	"errors"
	"os"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
)

// ExecAttachCtr execs and attaches to a container
func ExecAttachCtr(ctx context.Context, ctr *libpod.Container, execConfig *libpod.ExecConfig, streams *define.AttachStreams) (int, error) {
	return -1, errors.New("not implemented ExecAttachCtr")
}

// StartAttachCtr starts and (if required) attaches to a container
// if you change the signature of this function from os.File to io.Writer, it will trigger a downstream
// error. we may need to just lint disable this one.
func StartAttachCtr(ctx context.Context, ctr *libpod.Container, stdout, stderr, stdin *os.File, detachKeys string, sigProxy bool, startContainer bool) error { //nolint: interfacer
	return errors.New("not implemented StartAttachCtr")
}
