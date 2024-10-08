//go:build freebsd
// +build freebsd

package define

const (
	// TypeBind is the type for mounting host dir
	TypeBind = "nullfs"

	// TempDir is the default for storing temporary files
	TempDir = "/var/tmp"
)

var (
	// Mount potions for bind
	BindOptions = []string{}
)
