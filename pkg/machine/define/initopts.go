package define

import "net/url"

type InitOptions struct {
	CPUS               uint64
	DiskSize           uint64
	IgnitionPath       string
	ImagePath          string
	Volumes            []string
	VolumeDriver       string
	IsDefault          bool
	Memory             uint64
	Name               string
	TimeZone           string
	URI                url.URL
	Username           string
	ReExec             bool
	Rootful            bool
	UID                string // uid of the user that called machine
	UserModeNetworking *bool  // nil = use backend/system default, false = disable, true = enable
	USBs               []string
}
