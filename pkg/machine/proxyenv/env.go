package proxyenv

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/containers/common/libnetwork/etchosts"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v5/pkg/machine"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
	"github.com/sirupsen/logrus"
)

const proxySetupScriptTemplate = `#!/bin/bash

SYSTEMD_CONF=/etc/systemd/system.conf.d/default-env.conf
ENVD_CONF=/etc/environment.d/default-env.conf
PROFILE_CONF=/etc/profile.d/default-env.sh

mkdir -p /etc/profile.d /etc/environment.d /etc/systemd/system.conf.d/
rm -f $SYSTEMD_CONF $ENVD_CONF $PROFILE_CONF

echo "[Manager]" >> $SYSTEMD_CONF
for proxy in %s; do
	printf "DefaultEnvironment=\"%%s\"\n" "$proxy"  >> $SYSTEMD_CONF
	printf "%%q\n" "$proxy"  >> $ENVD_CONF
	printf "export %%q\n" "$proxy" >> $PROFILE_CONF
done

systemctl daemon-reload
`

func getProxyScript(isWSL bool) io.Reader {
	var envs []string
	for _, key := range config.ProxyEnv {
		if value, ok := os.LookupEnv(key); ok {
			// WSL does not use host.containers.internal as valid name for the VM.
			if !isWSL {
				value = strings.ReplaceAll(value, "127.0.0.1", etchosts.HostContainersInternal)
				value = strings.ReplaceAll(value, "localhost", etchosts.HostContainersInternal)
			}
			// %q to quote the value correctly
			envs = append(envs, fmt.Sprintf("%q", key+"="+value))
		}
	}

	script := fmt.Sprintf(proxySetupScriptTemplate, strings.Join(envs, " "))
	logrus.Tracef("Final environment variable setup script: %s", script)
	return strings.NewReader(script)
}

func ApplyProxies(mc *vmconfigs.MachineConfig) error {
	return machine.CommonSSHWithStdin("root", mc.SSH.IdentityPath, mc.Name, mc.SSH.Port, []string{"/usr/bin/bash"},
		getProxyScript(mc.WSLHypervisor != nil))
}
