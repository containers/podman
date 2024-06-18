package define

// This is used by the shim package to fetch the disk image for the guest.
// It can be overridden in InitOpts by external podman-machine users.
type ImagePuller interface {
	LocalPath() (*VMFile, error)
	SetSourceURI(sourceURI string)
	Download() error
}
