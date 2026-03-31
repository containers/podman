//go:build darwin

package cloudinit

import (
	"fmt"

	"go.podman.io/podman/v6/pkg/machine"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
)

func addMountsSupport(userData *UserData, mc *vmconfigs.MachineConfig) {
	userData.Mounts = make([][]string, 0, len(mc.Mounts))
	for _, m := range mc.Mounts {
		v := machine.MountToVirtIOFs(m)
		// cloud-init mounts format:
		// [fs_spec, fs_file, fs_vfstype, fs_mntops, fs-freq, fs-passno]
		// https://cloudinit.readthedocs.io/en/latest/reference/yaml_examples/mounts.html#mounts
		opts := fmt.Sprintf("context=%q", machine.NFSSELinuxContext)
		if v.ReadOnly {
			opts = opts + ",ro"
		}
		entry := []string{v.Tag, v.Target, "virtiofs", opts, "0", "0"}
		userData.Mounts = append(userData.Mounts, entry)
	}
}

func generateDefaultUserData(mc *vmconfigs.MachineConfig) ([]byte, error) {
	userData, err := defaultUserData(mc)
	if err != nil {
		return nil, err
	}

	// Preserve virtiofs mounts (with options) when using cloud-init
	if len(mc.Mounts) > 0 {
		addMountsSupport(userData, mc)
	}

	return userData.Marshal()
}

func generateUserData(mc *vmconfigs.MachineConfig) ([]byte, error) {
	// If user has not provided any custom user-data, generate default
	// otherwise use the provided one
	if mc.CloudInitConfig.UserData == nil {
		return generateDefaultUserData(mc)
	}

	customUserData, err := mc.CloudInitConfig.UserData.Read()
	if err != nil {
		return nil, err
	}

	// if user has provided a custom user-data but there are no mounts, return it as-is
	if len(mc.Mounts) == 0 {
		return customUserData, nil
	}

	// otherwise use the custom user-data and add the mounts
	generatedUserData := &UserData{}
	addMountsSupport(generatedUserData, mc)

	// if the user has provided a custom user-data
	// we need to merge our generated user data with the user's one
	// To do it we create a MIME multi-part archive
	// with both files
	return generatedUserData.MarshalMultiPart(customUserData)
}

func GetEmbeddedResources(_ *vmconfigs.MachineConfig) []EmbeddedResource {
	return nil
}
