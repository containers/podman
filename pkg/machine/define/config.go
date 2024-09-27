package define

import "os"

const (
	UserCertsTargetPath = "/etc/containers/certs.d"
	DefaultIdentityName = "machine"
	DefaultMachineName  = "podman-machine-default"
)

// MountTag is an identifier to mount a VirtioFS file system tag on a mount point in the VM.
// Ref: https://developer.apple.com/documentation/virtualization/running_intel_binaries_in_linux_vms_with_rosetta
const MountTag = "rosetta"

var (
	DefaultFilePerm os.FileMode = 0644
)

type CreateVMOpts struct {
	Name               string
	Dirs               *MachineDirs
	ReExec             bool
	UserModeNetworking bool
}

type MachineDirs struct {
	ConfigDir     *VMFile
	DataDir       *VMFile
	ImageCacheDir *VMFile
	RuntimeDir    *VMFile
}
