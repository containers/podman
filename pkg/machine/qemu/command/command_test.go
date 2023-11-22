package command

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/containers/common/libnetwork/etchosts"
	"github.com/containers/podman/v4/pkg/machine/define"
	"github.com/stretchr/testify/assert"
)

func TestPropagateHostEnv(t *testing.T) {
	tests := map[string]struct {
		value  string
		expect string
	}{
		"HTTP_PROXY": {
			"proxy",
			"equal",
		},
		"ftp_proxy": {
			"domain.com:8888",
			"equal",
		},
		"FTP_PROXY": {
			"proxy",
			"equal",
		},
		"NO_PROXY": {
			"localaddress",
			"equal",
		},
		"HTTPS_PROXY": {
			"",
			"unset",
		},
		"no_proxy": {
			"",
			"unset",
		},
		"http_proxy": {
			"127.0.0.1:8888",
			fmt.Sprintf("%s:8888", etchosts.HostContainersInternal),
		},
		"https_proxy": {
			"localhost:8888",
			fmt.Sprintf("%s:8888", etchosts.HostContainersInternal),
		},
		"SSL_CERT_FILE": {
			"/some/f=oo.cert",
			fmt.Sprintf("%s/f=oo.cert", define.UserCertsTargetPath),
		},
		"SSL_CERT_DIR": {
			"/some/my/certs",
			define.UserCertsTargetPath,
		},
	}

	for key, item := range tests {
		t.Setenv(key, item.value)
	}

	cmdLine := propagateHostEnv(make([]string, 0))

	assert.Len(t, cmdLine, 2)
	assert.Equal(t, "-fw_cfg", cmdLine[0])
	tokens := strings.Split(cmdLine[1], ",string=")
	decodeString, err := base64.StdEncoding.DecodeString(tokens[1])
	assert.NoError(t, err)

	// envsRawArr looks like: ["BAR=\"bar\"", "FOO=\"foo\""]
	envsRawArr := strings.Split(string(decodeString), "|")
	// envs looks like: {"BAR": "bar", "FOO": "foo"}
	envs := make(map[string]string)
	for _, env := range envsRawArr {
		item := strings.SplitN(env, "=", 2)
		envs[item[0]] = strings.Trim(item[1], "\"")
	}

	for key, test := range tests {
		switch test.expect {
		case "equal":
			assert.Equal(t, envs[key], test.value)
		case "unset":
			if _, ok := envs[key]; ok {
				t.Errorf("env %s should not be set", key)
			}
		default:
			assert.Equal(t, envs[key], test.expect)
		}
	}
}
