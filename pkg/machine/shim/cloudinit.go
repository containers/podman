package shim

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
)

func CmdLineCloudInitToConfig(files []string) (vmconfigs.CloudInitConfig, error) {
	config := vmconfigs.CloudInitConfig{
		UserData:      nil,
		MetaData:      nil,
		NetworkConfig: nil,
	}
	for _, file := range files {
		_, err := os.Stat(file)
		if errors.Is(err, os.ErrNotExist) {
			return config, fmt.Errorf("cloud-initfile %s not found: %w", file, err)
		}

		filename := filepath.Base(file)
		switch filename {
		case "user-data":
			config.UserData = &define.VMFile{
				Path: file,
			}
		case "meta-data":
			config.MetaData = &define.VMFile{
				Path: file,
			}
		case "network-config":
			config.NetworkConfig = &define.VMFile{
				Path: file,
			}
		}
	}
	return config, nil
}
