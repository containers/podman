// Package exec provides utilities for executing Open Container Initative runtime hooks.
package exec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	osexec "os/exec"
	"time"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
)

// DefaultPostKillTimeout is the recommended default post-kill timeout.
const DefaultPostKillTimeout = time.Duration(10) * time.Second

// Run executes the hook and waits for it to complete or for the
// context or hook-specified timeout to expire.
func Run(ctx context.Context, hook *rspec.Hook, state []byte, stdout io.Writer, stderr io.Writer, postKillTimeout time.Duration) (hookErr, err error) {
	cmd := osexec.Cmd{
		Path:   hook.Path,
		Args:   hook.Args,
		Env:    hook.Env,
		Stdin:  bytes.NewReader(state),
		Stdout: stdout,
		Stderr: stderr,
	}
	if cmd.Env == nil {
		cmd.Env = []string{}
	}

	if hook.Timeout != nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(*hook.Timeout)*time.Second)
		defer cancel()
	}

	err = cmd.Start()
	if err != nil {
		return err, err
	}
	exit := make(chan error, 1)
	go func() {
		exit <- cmd.Wait()
	}()

	select {
	case err = <-exit:
		return err, err
	case <-ctx.Done():
		cmd.Process.Kill()
		timer := time.NewTimer(postKillTimeout)
		defer timer.Stop()
		select {
		case <-timer.C:
			err = fmt.Errorf("failed to reap process within %s of the kill signal", postKillTimeout)
		case err = <-exit:
		}
		return err, ctx.Err()
	}
}
