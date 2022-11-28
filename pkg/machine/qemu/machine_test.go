//go:build (amd64 && !windows) || (arm64 && !windows)
// +build amd64,!windows arm64,!windows

package qemu

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/containers/podman/v4/pkg/machine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEditCmd(t *testing.T) {
	vm := new(MachineVM)
	vm.CmdLine = []string{"command", "-flag", "value"}

	vm.editCmdLine("-flag", "newvalue")
	vm.editCmdLine("-anotherflag", "anothervalue")

	require.Equal(t, vm.CmdLine, []string{"command", "-flag", "newvalue", "-anotherflag", "anothervalue"})
}

func TestPropagateHostEnv(t *testing.T) {
	t.Setenv("SSL_CERT_FILE", "/some/foo.cert")
	t.Setenv("SSL_CERT_DIR", "/some/my/certs")
	t.Setenv("HTTP_PROXY", "proxy")

	cmdLine := propagateHostEnv(make([]string, 0))

	assert.Len(t, cmdLine, 2)
	assert.Equal(t, "-fw_cfg", cmdLine[0])
	tokens := strings.Split(cmdLine[1], ",string=")
	decodeString, err := base64.StdEncoding.DecodeString(tokens[1])
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("HTTP_PROXY=\"proxy\"|SSL_CERT_FILE=\"%s/foo.cert\"|SSL_CERT_DIR=%q", machine.UserCertsTargetPath, machine.UserCertsTargetPath), string(decodeString))
}
