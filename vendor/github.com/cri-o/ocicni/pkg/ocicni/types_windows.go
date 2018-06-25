// +build windows

package ocicni

const (
	// DefaultNetDir is the place to look for CNI Network
	DefaultNetDir = "C:\\cni\\etc\\net.d"
	// DefaultCNIDir is the place to look for cni config files
	DefaultCNIDir = "C:\\cni\\bin"
	// VendorCNIDirTemplate is the template for looking up vendor specific cni config/executable files
	VendorCNIDirTemplate = "C:\\cni\\%s\\opt\\%s\\bin" // XXX(vbatts) Not sure what to do here ...
)
