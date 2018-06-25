// +build !windows

package ocicni

const (
	// DefaultNetDir is the place to look for CNI Network
	DefaultNetDir = "/etc/cni/net.d"
	// DefaultCNIDir is the place to look for cni config files
	DefaultCNIDir = "/opt/cni/bin"
	// VendorCNIDirTemplate is the template for looking up vendor specific cni config/executable files
	VendorCNIDirTemplate = "%s/opt/%s/bin"
)
