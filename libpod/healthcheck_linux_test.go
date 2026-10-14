//go:build !remote && systemd

package libpod

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	systemdCommon "go.podman.io/common/pkg/systemd"
	"go.podman.io/image/v5/manifest"
	"go.podman.io/podman/v6/libpod/define"
)

func TestDisableHealthCheckSystemd(t *testing.T) {
	t.Setenv("DISABLE_HC_SYSTEMD", "")
	if !systemdCommon.RunsOnSystemd() {
		t.Skip("requires systemd to exercise timer configuration checks")
	}
	active := &manifest.Schema2HealthConfig{Test: []string{"CMD", "true"}, Interval: time.Second}
	manual := &manifest.Schema2HealthConfig{Test: []string{"CMD", "true"}}
	disabled := &manifest.Schema2HealthConfig{Test: []string{"NONE"}, Interval: 30 * time.Second}
	startup := &define.StartupHealthCheck{Schema2HealthConfig: *active}
	tests := []struct {
		name      string
		regular   *manifest.Schema2HealthConfig
		startup   *define.StartupHealthCheck
		isStartup bool
		disabled  bool
	}{
		{name: "inherited NONE", regular: disabled, disabled: true},
		{name: "explicit NONE", regular: &manifest.Schema2HealthConfig{Test: []string{"NONE"}}, disabled: true},
		{name: "lowercase NONE", regular: &manifest.Schema2HealthConfig{Test: []string{"none"}, Interval: time.Second}, disabled: true},
		{name: "missing regular", disabled: true},
		{name: "empty regular command", regular: &manifest.Schema2HealthConfig{Interval: time.Second}, disabled: true},
		{name: "manual regular", regular: manual, disabled: true},
		{name: "active regular", regular: active},
		{name: "missing startup", regular: active, isStartup: true, disabled: true},
		{name: "manual startup", regular: active, startup: &define.StartupHealthCheck{Schema2HealthConfig: *manual}, isStartup: true, disabled: true},
		{name: "active startup", regular: active, startup: startup, isStartup: true},
		{name: "active startup with manual regular", regular: manual, startup: startup, isStartup: true},
		{name: "manual regular after startup", regular: manual, startup: startup, disabled: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctr := &Container{config: &ContainerConfig{}}
			ctr.config.HealthCheckConfig = tt.regular
			ctr.config.StartupHealthCheckConfig = tt.startup
			if !assert.Equal(t, tt.disabled, ctr.disableHealthCheckSystemd(tt.isStartup)) {
				return
			}
			if tt.disabled {
				assert.NoError(t, ctr.createTimer("1s", tt.isStartup))
				assert.NoError(t, ctr.startTimer(tt.isStartup))
				assert.NoError(t, ctr.removeTransientFiles(context.Background(), tt.isStartup, ""))
			}
		})
	}
}

func TestHCUnitName(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		isStartup bool
		bare      bool
		expected  string
	}{
		{
			name:      "uses libpod-healthcheck prefix",
			id:        "a1b2c3d4e5f67890123456789abcdef0123456789abcdef0123456789abcdef",
			isStartup: false,
			bare:      false,
			expected:  `^libpod-healthcheck-a1b2c3d4e5f67890123456789abcdef0123456789abcdef0123456789abcdef-[0-9a-f]+$`,
		},
		{
			name:      "adds startup suffix",
			id:        "a1b2c3d4e5f67890123456789abcdef0123456789abcdef0123456789abcdef",
			isStartup: true,
			bare:      false,
			expected:  `^libpod-healthcheck-a1b2c3d4e5f67890123456789abcdef0123456789abcdef0123456789abcdef-startup-[0-9a-f]+$`,
		},
		{
			name:      "omits random suffix when bare is true",
			id:        "a1b2c3d4e5f67890123456789abcdef0123456789abcdef0123456789abcdef",
			isStartup: false,
			bare:      true,
			expected:  `^libpod-healthcheck-a1b2c3d4e5f67890123456789abcdef0123456789abcdef0123456789abcdef$`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctr := &Container{
				config: &ContainerConfig{},
			}

			ctr.config.ID = tt.id

			assert.Regexp(t, tt.expected, ctr.hcUnitName(tt.isStartup, tt.bare))
		})
	}
}
