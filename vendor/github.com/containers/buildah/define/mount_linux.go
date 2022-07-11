//go:build linux
// +build linux

package define

const (
	// TypeBind is the type for mounting host dir
	TypeBind = "bind"

	// TempDir is the default for storing temporary files
	TempDir = "/dev/shm"
)

var (
	// Mount potions for bind
	BindOptions = []string{"bind"}
)
