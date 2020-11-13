// +build !linux,!darwin

package util

import (
	"os"
)

type HardlinkChecker struct {
}

func (h *HardlinkChecker) Check(fi os.FileInfo) string {
	return ""
}
func (h *HardlinkChecker) Add(fi os.FileInfo, name string) {
}

func UID(st os.FileInfo) int {
	return 0
}

func GID(st os.FileInfo) int {
	return 0
}
