package command

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/common/libnetwork/etchosts"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/pkg/machine/define"
)

func GetProxyVariables() map[string]string {
	proxyOpts := make(map[string]string)
	for _, variable := range config.ProxyEnv {
		if value, ok := os.LookupEnv(variable); ok {
			if value == "" {
				continue
			}

			v := strings.ReplaceAll(value, "127.0.0.1", etchosts.HostContainersInternal)
			v = strings.ReplaceAll(v, "localhost", etchosts.HostContainersInternal)
			proxyOpts[variable] = v
		}
	}
	return proxyOpts
}

// PropagateHostEnv is here for providing the ability to propagate
// proxy and SSL settings (e.g. HTTP_PROXY and others) on a start
// and avoid a need of re-creating/re-initiating a VM
func PropagateHostEnv(cmdLine QemuCmd) QemuCmd {
	varsToPropagate := make([]string, 0)

	for k, v := range GetProxyVariables() {
		varsToPropagate = append(varsToPropagate, fmt.Sprintf("%s=%q", k, v))
	}

	if sslCertFile, ok := os.LookupEnv("SSL_CERT_FILE"); ok {
		pathInVM := filepath.Join(define.UserCertsTargetPath, filepath.Base(sslCertFile))
		varsToPropagate = append(varsToPropagate, fmt.Sprintf("%s=%q", "SSL_CERT_FILE", pathInVM))
	}

	if _, ok := os.LookupEnv("SSL_CERT_DIR"); ok {
		varsToPropagate = append(varsToPropagate, fmt.Sprintf("%s=%q", "SSL_CERT_DIR", define.UserCertsTargetPath))
	}

	if len(varsToPropagate) > 0 {
		prefix := "name=opt/com.coreos/environment,string="
		envVarsJoined := strings.Join(varsToPropagate, "|")
		fwCfgArg := prefix + base64.StdEncoding.EncodeToString([]byte(envVarsJoined))
		return append(cmdLine, "-fw_cfg", fwCfgArg)
	}

	return cmdLine
}
