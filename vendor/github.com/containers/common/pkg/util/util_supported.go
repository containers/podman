//go:build linux || darwin || freebsd
// +build linux darwin freebsd

package util

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/containers/storage/pkg/homedir"
	"github.com/containers/storage/pkg/unshare"
	"github.com/sirupsen/logrus"
	terminal "golang.org/x/term"
)

var (
	rootlessRuntimeDirOnce sync.Once
	rootlessRuntimeDir     string
)

// isWriteableOnlyByOwner checks that the specified permission mask allows write
// access only to the owner.
func isWriteableOnlyByOwner(perm os.FileMode) bool {
	return (perm & 0o722) == 0o700
}

// GetRuntimeDir returns the runtime directory
func GetRuntimeDir() (string, error) {
	var rootlessRuntimeDirError error

	rootlessRuntimeDirOnce.Do(func() {
		runtimeDir, err := homedir.GetRuntimeDir()
		if err != nil {
			logrus.Debug(err)
		}
		if runtimeDir != "" {
			st, err := os.Stat(runtimeDir)
			if err != nil {
				rootlessRuntimeDirError = err
				return
			}
			if int(st.Sys().(*syscall.Stat_t).Uid) != os.Geteuid() {
				rootlessRuntimeDirError = fmt.Errorf("XDG_RUNTIME_DIR directory %q is not owned by the current user", runtimeDir)
				return
			}
		}
		uid := fmt.Sprintf("%d", unshare.GetRootlessUID())
		if runtimeDir == "" {
			tmpDir := filepath.Join("/run", "user", uid)
			if err := os.MkdirAll(tmpDir, 0o700); err != nil {
				logrus.Debugf("unable to make temp dir: %v", err)
			}
			st, err := os.Stat(tmpDir)
			if err == nil && int(st.Sys().(*syscall.Stat_t).Uid) == os.Geteuid() && isWriteableOnlyByOwner(st.Mode().Perm()) {
				runtimeDir = tmpDir
			}
		}
		if runtimeDir == "" {
			tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("podman-run-%s", uid))
			if err := os.MkdirAll(tmpDir, 0o700); err != nil {
				logrus.Debugf("unable to make temp dir %v", err)
			}
			st, err := os.Stat(tmpDir)
			if err == nil && int(st.Sys().(*syscall.Stat_t).Uid) == os.Geteuid() && isWriteableOnlyByOwner(st.Mode().Perm()) {
				runtimeDir = tmpDir
			}
		}
		if runtimeDir == "" {
			home := os.Getenv("HOME")
			if home == "" {
				rootlessRuntimeDirError = errors.New("neither XDG_RUNTIME_DIR nor HOME was set non-empty")
				return
			}
			resolvedHome, err := filepath.EvalSymlinks(home)
			if err != nil {
				rootlessRuntimeDirError = fmt.Errorf("cannot resolve home: %w", err)
				return
			}
			runtimeDir = filepath.Join(resolvedHome, "rundir")
		}
		rootlessRuntimeDir = runtimeDir
	})

	if rootlessRuntimeDirError != nil {
		return "", rootlessRuntimeDirError
	}
	return rootlessRuntimeDir, nil
}

// ReadPassword reads a password from the terminal without echo.
func ReadPassword(fd int) ([]byte, error) {
	// Store and restore the terminal status on interruptions to
	// avoid that the terminal remains in the password state
	// This is necessary as for https://github.com/golang/go/issues/31180

	oldState, err := terminal.GetState(fd)
	if err != nil {
		return make([]byte, 0), err
	}

	type Buffer struct {
		Buffer []byte
		Error  error
	}
	errorChannel := make(chan Buffer, 1)

	// SIGINT and SIGTERM restore the terminal, otherwise the no-echo mode would remain intact
	interruptChannel := make(chan os.Signal, 1)
	signal.Notify(interruptChannel, syscall.SIGINT, syscall.SIGTERM)
	defer func() {
		signal.Stop(interruptChannel)
		close(interruptChannel)
	}()
	go func() {
		for range interruptChannel {
			if oldState != nil {
				_ = terminal.Restore(fd, oldState)
			}
			errorChannel <- Buffer{Buffer: make([]byte, 0), Error: ErrInterrupt}
		}
	}()

	go func() {
		buf, err := terminal.ReadPassword(fd)
		errorChannel <- Buffer{Buffer: buf, Error: err}
	}()

	buf := <-errorChannel
	return buf.Buffer, buf.Error
}
