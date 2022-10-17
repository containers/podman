package machine

import (
	"fmt"
	"regexp"
	"strings"
)

// Given a slice of Mounts, build the necessary systemd units for the mount
func buildMountUnits(mounts []Mount) []Unit {
	mountUnits := []Unit{}

	// This creates the path.mount file, which actually mounts the target.
	systemdMountTemplate := `[Unit]
Description=UserVolume Mount

[Mount]
What=%s
Where=%s
Type=%s
Options=defaults
DirectoryMode=0755

[Install]
WantedBy=default.target`

	// This creates the path.service file, which is used to ensure the path
	// to mount the volume exists in the filesystem.
	volumeMountPointTemplate := `[Unit]
Description=Ensures %s Mountpoint
Before=%s

[Service]
Type=oneshot
ExecStart=sh -c 'chattr -i /'
ExecStart=sh -c 'mkdir -p %s'
ExecStart=sh -c 'chattr +i /'
RemainAfterExit=yes

[Install]
WantedBy=default.target`

	for _, mount := range mounts {
		systemdName := fsPathToMountName(mount.Target)
		mountUnitName := fmt.Sprintf("%s.mount", systemdName)
		serviceUnitName := fmt.Sprintf("%s.service", systemdName)

		if !strings.HasPrefix(mount.Target, "/home") && !strings.HasPrefix(mount.Target, "/mnt") {
			mkdirUnit := Unit{
				Enabled: boolToPtr(true),
				Name:    serviceUnitName,
				Contents: strToPtr(fmt.Sprintf(
					volumeMountPointTemplate,
					mount.Tag,
					mountUnitName,
					mount.Target,
					mountUnitName,
				)),
			}

			mountUnits = append(mountUnits, mkdirUnit)
		}

		unit := Unit{
			Enabled: boolToPtr(true),
			Name:    mountUnitName,
			Contents: strToPtr(fmt.Sprintf(
				systemdMountTemplate,
				mount.Tag,
				mount.Target,
				mount.Type,
			)),
		}

		mountUnits = append(mountUnits, unit)
	}
	return mountUnits
}

// Builds the systemd service name from the given mount path
func fsPathToMountName(path string) string {
	// according to the systemd docs we need
	// to replace the slashes with dashes
	// and drop proceeding slashes so
	// /var/lib/podman would become
	// var-lib-podman
	if path[0:1] == "/" {
		path = path[1:]
	}
	// also lets drop any trailing slashes
	if path[len(path)-1:] == "/" {
		path = path[0 : len(path)-1]
	}

	// replace with dashes
	re := regexp.MustCompile(`/+`)
	path = re.ReplaceAllString(path, "-")

	return path
}
