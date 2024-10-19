//go:build linux && cgo
// +build linux,cgo

package loopback

import (
	"fmt"
	"os"
	"syscall"

	"github.com/sirupsen/logrus"
)

func getLoopbackBackingFile(file *os.File) (uint64, uint64, error) {
	loopInfo, err := ioctlLoopGetStatus64(file.Fd())
	if err != nil {
		logrus.Errorf("Get loopback backing file: %v", err)
		return 0, 0, ErrGetLoopbackBackingFile
	}
	return loopInfo.loDevice, loopInfo.loInode, nil
}

// SetCapacity reloads the size for the loopback device.
func SetCapacity(file *os.File) error {
	if err := ioctlLoopSetCapacity(file.Fd(), 0); err != nil {
		logrus.Errorf("loopbackSetCapacity: %s", err)
		return ErrSetCapacity
	}
	return nil
}

// FindLoopDeviceFor returns a loopback device file for the specified file which
// is backing file of a loop back device.
func FindLoopDeviceFor(file *os.File) *os.File {
	stat, err := file.Stat()
	if err != nil {
		return nil
	}
	targetInode := stat.Sys().(*syscall.Stat_t).Ino
	targetDevice := stat.Sys().(*syscall.Stat_t).Dev

	for i := 0; true; i++ {
		path := fmt.Sprintf("/dev/loop%d", i)

		file, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}

			// Ignore all errors until the first not-exist
			// we want to continue looking for the file
			continue
		}

		dev, inode, err := getLoopbackBackingFile(file)
		if err == nil && dev == uint64(targetDevice) && inode == targetInode {
			return file
		}
		file.Close()
	}

	return nil
}
