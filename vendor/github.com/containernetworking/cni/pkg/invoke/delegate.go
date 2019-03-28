// Copyright 2016 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package invoke

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containernetworking/cni/pkg/types"
)

func delegateCommon(expectedCommand, delegatePlugin string, exec Exec) (string, Exec, error) {
	if exec == nil {
		exec = defaultExec
	}

	if os.Getenv("CNI_COMMAND") != expectedCommand {
		return "", nil, fmt.Errorf("CNI_COMMAND is not " + expectedCommand)
	}

	paths := filepath.SplitList(os.Getenv("CNI_PATH"))
	pluginPath, err := exec.FindInPath(delegatePlugin, paths)
	if err != nil {
		return "", nil, err
	}

	return pluginPath, exec, nil
}

// DelegateAdd calls the given delegate plugin with the CNI ADD action and
// JSON configuration
func DelegateAdd(ctx context.Context, delegatePlugin string, netconf []byte, exec Exec) (types.Result, error) {
	pluginPath, realExec, err := delegateCommon("ADD", delegatePlugin, exec)
	if err != nil {
		return nil, err
	}

	return ExecPluginWithResult(ctx, pluginPath, netconf, ArgsFromEnv(), realExec)
}

// DelegateCheck calls the given delegate plugin with the CNI CHECK action and
// JSON configuration
func DelegateCheck(ctx context.Context, delegatePlugin string, netconf []byte, exec Exec) error {
	pluginPath, realExec, err := delegateCommon("CHECK", delegatePlugin, exec)
	if err != nil {
		return err
	}

	return ExecPluginWithoutResult(ctx, pluginPath, netconf, ArgsFromEnv(), realExec)
}

// DelegateDel calls the given delegate plugin with the CNI DEL action and
// JSON configuration
func DelegateDel(ctx context.Context, delegatePlugin string, netconf []byte, exec Exec) error {
	pluginPath, realExec, err := delegateCommon("DEL", delegatePlugin, exec)
	if err != nil {
		return err
	}

	return ExecPluginWithoutResult(ctx, pluginPath, netconf, ArgsFromEnv(), realExec)
}
