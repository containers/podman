// +build !linux

package adapter

import (
	"context"
	"os"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
)

// ExecAttachCtr execs and attaches to a container
func ExecAttachCtr(ctx context.Context, ctr *libpod.Container, tty, privileged bool, env map[string]string, cmd []string, user, workDir string, streams *define.AttachStreams, preserveFDs uint, detachKeys string) (int, error) {
	return -1, define.ErrNotImplemented
}

// StartAttachCtr starts and (if required) attaches to a container
// if you change the signature of this function from os.File to io.Writer, it will trigger a downstream
// error. we may need to just lint disable this one.
func StartAttachCtr(ctx context.Context, ctr *libpod.Container, stdout, stderr, stdin *os.File, detachKeys string, sigProxy bool, startContainer bool, recursive bool) error { //nolint-interfacer
	return define.ErrNotImplemented
}
