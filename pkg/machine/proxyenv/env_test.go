package proxyenv

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getProxyScript(t *testing.T) {
	type env struct {
		name  string
		value string
	}
	type args struct {
		isWSL bool
		envs  []env
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "all vars set",
			args: args{
				isWSL: false,
				envs: []env{
					{
						name:  "http_proxy",
						value: "proxy1",
					},
					{
						name:  "https_proxy",
						value: "sproxy1",
					},
					{
						name:  "no_proxy",
						value: "no1,no2",
					},
				},
			},
			want: `#!/bin/bash

SYSTEMD_CONF=/etc/systemd/system.conf.d/default-env.conf
ENVD_CONF=/etc/environment.d/default-env.conf
PROFILE_CONF=/etc/profile.d/default-env.sh

mkdir -p /etc/profile.d /etc/environment.d /etc/systemd/system.conf.d/
rm -f $SYSTEMD_CONF $ENVD_CONF $PROFILE_CONF

echo "[Manager]" >> $SYSTEMD_CONF
for proxy in "http_proxy=proxy1" "https_proxy=sproxy1" "no_proxy=no1,no2"; do
	printf "DefaultEnvironment=\"%s\"\n" "$proxy"  >> $SYSTEMD_CONF
	printf "%q\n" "$proxy"  >> $ENVD_CONF
	printf "export %q\n" "$proxy" >> $PROFILE_CONF
done

systemctl daemon-reload
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, e := range tt.args.envs {
				t.Setenv(e.name, e.value)
			}
			got := getProxyScript(tt.args.isWSL)
			buf := new(bytes.Buffer)
			_, err := buf.ReadFrom(got)
			assert.NoError(t, err)
			str := buf.String()
			assert.Equal(t, tt.want, str)
		})
	}
}
