package libpod

import (
	"net"
	"strings"

	"github.com/containers/storage/pkg/mount"
	"github.com/pkg/errors"
)

// Volume is the type used to create named volumes
// TODO: all volumes should be created using this and the Volume API
type Volume struct {
	config *VolumeConfig

	valid   bool
	runtime *Runtime
}

// VolumeConfig holds the volume's config information
//easyjson:json
type VolumeConfig struct {
	// Name of the volume
	Name string `json:"name"`

	Labels        map[string]string `json:"labels"`
	MountPoint    string            `json:"mountPoint"`
	Driver        string            `json:"driver"`
	Options       map[string]string `json:"options"`
	Scope         string            `json:"scope"`
	IsCtrSpecific bool              `json:"ctrSpecific"`
	UID           int               `json:"uid"`
	GID           int               `json:"gid"`
}

// Name retrieves the volume's name
func (v *Volume) Name() string {
	return v.config.Name
}

// Labels returns the volume's labels
func (v *Volume) Labels() map[string]string {
	labels := make(map[string]string)
	for key, value := range v.config.Labels {
		labels[key] = value
	}
	return labels
}

// MountPoint returns the volume's mountpoint on the host
func (v *Volume) MountPoint() string {
	return v.config.MountPoint
}

// Driver returns the volume's driver
func (v *Volume) Driver() string {
	return v.config.Driver
}

// Options return the volume's options
func (v *Volume) Options() map[string]string {
	options := make(map[string]string)
	for key, value := range v.config.Options {
		options[key] = value
	}

	return options
}

// Scope returns the scope of the volume
func (v *Volume) Scope() string {
	return v.config.Scope
}

// IsCtrSpecific returns whether this volume was created specifically for a
// given container. Images with this set to true will be removed when the
// container is removed with the Volumes parameter set to true.
func (v *Volume) IsCtrSpecific() bool {
	return v.config.IsCtrSpecific
}

// Mount the volume
func (v *Volume) Mount() error {
	if v.MountPoint() == "" {
		return errors.Errorf("missing device in volume options")
	}
	mounted, err := mount.Mounted(v.MountPoint())
	if err != nil {
		return errors.Wrapf(err, "failed to determine if %v is mounted", v.Name())
	}
	if mounted {
		return nil
	}
	options := v.Options()
	if len(options) == 0 {
		return errors.Errorf("volume %v is not mountable, no options available", v.Name())
	}
	mountOpts := options["o"]
	device := options["device"]
	if options["type"] == "nfs" {
		if addrValue := getAddress(mountOpts); addrValue != "" && net.ParseIP(addrValue).To4() == nil {
			ipAddr, err := net.ResolveIPAddr("ip", addrValue)
			if err != nil {
				return errors.Wrapf(err, "error resolving passed in nfs address")
			}
			mountOpts = strings.Replace(mountOpts, "addr="+addrValue, "addr="+ipAddr.String(), 1)
		}
		if device[0] != ':' {
			device = ":" + device
		}
	}
	err = mount.Mount(device, v.MountPoint(), options["type"], mountOpts)
	return errors.Wrap(err, "failed to mount local volume")
}

// Unmount the volume from the system
func (v *Volume) Unmount() error {
	if v.MountPoint() == "" {
		return errors.Errorf("missing device in volume options")
	}
	return mount.Unmount(v.MountPoint())
}

// getAddress finds out address/hostname from options
func getAddress(opts string) string {
	optsList := strings.Split(opts, ",")
	for i := 0; i < len(optsList); i++ {
		if strings.HasPrefix(optsList[i], "addr=") {
			addr := strings.SplitN(optsList[i], "=", 2)[1]
			return addr
		}
	}
	return ""
}
