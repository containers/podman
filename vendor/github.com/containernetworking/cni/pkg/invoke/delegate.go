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
	"fmt"
	"os"
	"path/filepath"

	"github.com/containernetworking/cni/pkg/types"
)

func delegateAddOrGet(command, delegatePlugin string, netconf []byte, exec Exec) (types.Result, error) {
	if exec == nil {
		exec = defaultExec
	}

	paths := filepath.SplitList(os.Getenv("CNI_PATH"))
	pluginPath, err := exec.FindInPath(delegatePlugin, paths)
	if err != nil {
		return nil, err
	}

	return ExecPluginWithResult(pluginPath, netconf, ArgsFromEnv(), exec)
}

// DelegateAdd calls the given delegate plugin with the CNI ADD action and
// JSON configuration
func DelegateAdd(delegatePlugin string, netconf []byte, exec Exec) (types.Result, error) {
	if os.Getenv("CNI_COMMAND") != "ADD" {
		return nil, fmt.Errorf("CNI_COMMAND is not ADD")
	}
	return delegateAddOrGet("ADD", delegatePlugin, netconf, exec)
}

// DelegateGet calls the given delegate plugin with the CNI GET action and
// JSON configuration
func DelegateGet(delegatePlugin string, netconf []byte, exec Exec) (types.Result, error) {
	if os.Getenv("CNI_COMMAND") != "GET" {
		return nil, fmt.Errorf("CNI_COMMAND is not GET")
	}
	return delegateAddOrGet("GET", delegatePlugin, netconf, exec)
}

// DelegateDel calls the given delegate plugin with the CNI DEL action and
// JSON configuration
func DelegateDel(delegatePlugin string, netconf []byte, exec Exec) error {
	if exec == nil {
		exec = defaultExec
	}

	if os.Getenv("CNI_COMMAND") != "DEL" {
		return fmt.Errorf("CNI_COMMAND is not DEL")
	}

	paths := filepath.SplitList(os.Getenv("CNI_PATH"))
	pluginPath, err := exec.FindInPath(delegatePlugin, paths)
	if err != nil {
		return err
	}

	return ExecPluginWithoutResult(pluginPath, netconf, ArgsFromEnv(), exec)
}
