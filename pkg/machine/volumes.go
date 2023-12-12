package machine

import (
	"strings"

	"github.com/containers/podman/v4/pkg/machine/vmconfigs"
)

type Volume interface {
	Kind() VolumeKind
}

type VolumeKind string

var (
	VirtIOFsVk VolumeKind = "virtiofs"
	NinePVk    VolumeKind = "9p"
)

type VirtIoFs struct {
	VolumeKind
	ReadOnly bool
	Source   string
	Tag      string
	Target   string
}

func (v VirtIoFs) Kind() string {
	return string(VirtIOFsVk)
}

// unitName is the fq path where /'s are replaced with -'s
func (v VirtIoFs) unitName() string {
	// delete the leading -
	unit := strings.ReplaceAll(v.Target, "/", "-")
	if strings.HasPrefix(unit, "-") {
		return unit[1:]
	}
	return unit
}

func (v VirtIoFs) ToMount() vmconfigs.Mount {
	return vmconfigs.Mount{
		ReadOnly: v.ReadOnly,
		Source:   v.Source,
		Tag:      v.Tag,
		Target:   v.Target,
		Type:     v.Kind(),
	}
}

// NewVirtIoFsMount describes a machine volume mount for virtio-fs.  With virtio-fs
// the source/target are described as a "shared dir".  With this style of volume mount
// the Tag is used as the descriptor value for the mount (in Linux).
func NewVirtIoFsMount(src, target string, readOnly bool) VirtIoFs {
	vfs := VirtIoFs{
		ReadOnly: readOnly,
		Source:   src,
		Target:   target,
	}
	vfs.Tag = vfs.unitName()
	return vfs
}
