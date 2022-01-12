// Copyright 2016 CNI authors
// Copyright 2021 Podman authors
//
// This code has been originally copied from github.com/containernetworking/cni
// but has been changed to better fit the Podman use case.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build linux

package cni

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/containernetworking/cni/pkg/invoke"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containers/storage/pkg/unshare"
)

type cniExec struct {
	version.PluginDecoder
}

type cniPluginError struct {
	plugin  string
	Code    uint   `json:"code"`
	Msg     string `json:"msg"`
	Details string `json:"details,omitempty"`
}

// Error returns a nicely formatted error message for the cni plugin errors.
func (e *cniPluginError) Error() string {
	err := fmt.Sprintf("cni plugin %s failed", e.plugin)
	if e.Msg != "" {
		err = fmt.Sprintf("%s: %s", err, e.Msg)
	} else if e.Code > 0 {
		err = fmt.Sprintf("%s with error code %d", err, e.Code)
	}
	if e.Details != "" {
		err = fmt.Sprintf("%s: %s", err, e.Details)
	}
	return err
}

// ExecPlugin execute the cni plugin. Returns the stdout of the plugin or an error.
func (e *cniExec) ExecPlugin(ctx context.Context, pluginPath string, stdinData []byte, environ []string) ([]byte, error) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	c := exec.CommandContext(ctx, pluginPath)
	c.Env = environ
	c.Stdin = bytes.NewBuffer(stdinData)
	c.Stdout = stdout
	c.Stderr = stderr

	// The dnsname plugin tries to use XDG_RUNTIME_DIR to store files.
	// podman run will have XDG_RUNTIME_DIR set and thus the cni plugin can use
	// it. The problem is that XDG_RUNTIME_DIR is unset for the conmon process
	// for rootful users. This causes issues since the cleanup process is spawned
	// by conmon and thus not have XDG_RUNTIME_DIR set to same value as podman run.
	// Because of it dnsname will not find the config files and cannot correctly cleanup.
	// To fix this we should also unset XDG_RUNTIME_DIR for the cni plugins as rootful.
	if !unshare.IsRootless() {
		c.Env = append(c.Env, "XDG_RUNTIME_DIR=")
	}

	err := c.Run()
	if err != nil {
		return nil, annotatePluginError(err, pluginPath, stdout.Bytes(), stderr.Bytes())
	}
	return stdout.Bytes(), nil
}

// annotatePluginError parses the common cni plugin error json.
func annotatePluginError(err error, plugin string, stdout, stderr []byte) error {
	pluginName := filepath.Base(plugin)
	emsg := cniPluginError{
		plugin: pluginName,
	}
	if len(stdout) == 0 {
		if len(stderr) == 0 {
			emsg.Msg = err.Error()
		} else {
			emsg.Msg = string(stderr)
		}
	} else if perr := json.Unmarshal(stdout, &emsg); perr != nil {
		emsg.Msg = fmt.Sprintf("failed to unmarshal error message %q: %v", string(stdout), perr)
	}
	return &emsg
}

// FindInPath finds the plugin in the given paths.
func (e *cniExec) FindInPath(plugin string, paths []string) (string, error) {
	return invoke.FindInPath(plugin, paths)
}
