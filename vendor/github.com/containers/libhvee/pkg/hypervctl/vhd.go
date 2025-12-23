//go:build windows

package hypervctl

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/containers/libhvee/pkg/powershell"
	"go.podman.io/common/pkg/strongunits"
)

// ResizeDisk takes a diskPath and strongly typed new size and uses powershell
// to change its size.  There is no error protection for trying to size a disk
// smaller than the current size.
func ResizeDisk(diskPath string, newSize strongunits.GiB) error {
	cmd := `Resize-VHD -Path ` + diskPath + ` -SizeBytes ` + strconv.FormatInt(int64(newSize.ToBytes()), 10)
	_, stderr, err := powershell.Execute(cmd)
	if err != nil {
		return fmt.Errorf("failed to resize disk: %w", NewPSError(stderr))
	}
	return nil
}

func GetDiskSize(diskPath string) (strongunits.B, error) {

	cmd := `Get-VHD -Path "` + diskPath + `" | Select-Object -ExpandProperty Size`
	sizeStr, stderr, err := powershell.Execute(cmd)
	if err != nil {
		return 0, fmt.Errorf("failed to get disk size: %w", NewPSError(stderr))
	}
	sizeStr = strings.TrimSpace(sizeStr)
	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse disk size: %w", err)
	}
	return strongunits.B(size), nil
}
