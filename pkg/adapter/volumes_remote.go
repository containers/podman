// +build remoteclient

package adapter

// Name returns the name of the volume
func (v *Volume) Name() string {
	return v.config.Name
}

//Labels returns the labels for a volume
func (v *Volume) Labels() map[string]string {
	return v.config.Labels
}

// Driver returns the driver for the volume
func (v *Volume) Driver() string {
	return v.config.Driver
}

// Options returns the options a volume was created with
func (v *Volume) Options() map[string]string {
	return v.config.Options
}

// MountPath returns the path the volume is mounted to
func (v *Volume) MountPoint() (string, error) {
	return v.config.MountPoint, nil
}

// Scope returns the scope for an adapter.volume
func (v *Volume) Scope() string {
	return "local"
}
