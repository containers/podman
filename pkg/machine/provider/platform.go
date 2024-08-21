package provider

import (
	"github.com/containers/podman/v5/pkg/machine/define"
)

func InstalledProviders() ([]define.VMType, error) {
	installedTypes := []define.VMType{}
	providers := GetAll()
	for _, p := range providers {
		installed, err := IsInstalled(p.VMType())
		if err != nil {
			return nil, err
		}
		if installed {
			installedTypes = append(installedTypes, p.VMType())
		}
	}
	return installedTypes, nil
}
