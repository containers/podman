//go:build !windows

package ocipull

import (
	"path/filepath"

	"github.com/containers/common/pkg/config"
	"github.com/containers/storage/pkg/homedir"
)

func localPolicyOverwrites() []string {
	var dirs []string
	if p, err := homedir.GetConfigHome(); err == nil {
		dirs = append(dirs, filepath.Join(p, "containers", policyfile))
	}
	dirs = append(dirs, config.DefaultSignaturePolicyPath)
	return dirs
}
