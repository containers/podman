// +build linux,cgo

package loopback

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/sirupsen/logrus"
)

// Loopback related errors
var (
	ErrAttachLoopbackDevice   = errors.New("loopback attach failed")
	ErrGetLoopbackBackingFile = errors.New("Unable to get loopback backing file")
	ErrSetCapacity            = errors.New("Unable set loopback capacity")
)

func stringToLoopName(src string) [LoNameSize]uint8 {
	var dst [LoNameSize]uint8
	copy(dst[:], src[:])
	return dst
}

func getNextFreeLoopbackIndex() (int, error) {
	f, err := os.OpenFile("/dev/loop-control", os.O_RDONLY, 0644)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	index, err := ioctlLoopCtlGetFree(f.Fd())
	if index < 0 {
		index = 0
	}
	return index, err
}

func openNextAvailableLoopback(index int, sparseName string, sparseFile *os.File) (loopFile *os.File, err error) {
	// Read information about the loopback file.
	var st syscall.Stat_t
	err = syscall.Fstat(int(sparseFile.Fd()), &st)
	if err != nil {
		logrus.Errorf("Reading information about loopback file %s: %v", sparseName, err)
		return nil, ErrAttachLoopbackDevice
	}

	// Start looking for a free /dev/loop
	for {
		target := fmt.Sprintf("/dev/loop%d", index)
		index++

		fi, err := os.Stat(target)
		if err != nil {
			if os.IsNotExist(err) {
				logrus.Error("There are no more loopback devices available.")
			}
			return nil, ErrAttachLoopbackDevice
		}

		if fi.Mode()&os.ModeDevice != os.ModeDevice {
			logrus.Errorf("Loopback device %s is not a block device.", target)
			continue
		}

		// OpenFile adds O_CLOEXEC
		loopFile, err = os.OpenFile(target, os.O_RDWR, 0644)
		if err != nil {
			logrus.Errorf("Opening loopback device: %s", err)
			return nil, ErrAttachLoopbackDevice
		}

		// Try to attach to the loop file
		if err := ioctlLoopSetFd(loopFile.Fd(), sparseFile.Fd()); err != nil {
			loopFile.Close()

			// If the error is EBUSY, then try the next loopback
			if err != syscall.EBUSY {
				logrus.Errorf("Cannot set up loopback device %s: %s", target, err)
				return nil, ErrAttachLoopbackDevice
			}

			// Otherwise, we keep going with the loop
			continue
		}

		// Check if the loopback driver and underlying filesystem agree on the loopback file's
		// device and inode numbers.
		dev, ino, err := getLoopbackBackingFile(loopFile)
		if err != nil {
			logrus.Errorf("Getting loopback backing file: %s", err)
			return nil, ErrGetLoopbackBackingFile
		}
		if dev != uint64(st.Dev) || ino != st.Ino {
			logrus.Errorf("Loopback device and filesystem disagree on device/inode for %q: %#x(%d):%#x(%d) vs %#x(%d):%#x(%d)", sparseName, dev, dev, ino, ino, st.Dev, st.Dev, st.Ino, st.Ino)
		}

		// In case of success, we finished. Break the loop.
		break
	}

	// This can't happen, but let's be sure
	if loopFile == nil {
		logrus.Errorf("Unreachable code reached! Error attaching %s to a loopback device.", sparseFile.Name())
		return nil, ErrAttachLoopbackDevice
	}

	return loopFile, nil
}

// AttachLoopDevice attaches the given sparse file to the next
// available loopback device. It returns an opened *os.File.
func AttachLoopDevice(sparseName string) (loop *os.File, err error) {

	// Try to retrieve the next available loopback device via syscall.
	// If it fails, we discard error and start looping for a
	// loopback from index 0.
	startIndex, err := getNextFreeLoopbackIndex()
	if err != nil {
		logrus.Debugf("Error retrieving the next available loopback: %s", err)
	}

	// OpenFile adds O_CLOEXEC
	sparseFile, err := os.OpenFile(sparseName, os.O_RDWR, 0644)
	if err != nil {
		logrus.Errorf("Opening sparse file: %v", err)
		return nil, ErrAttachLoopbackDevice
	}
	defer sparseFile.Close()

	loopFile, err := openNextAvailableLoopback(startIndex, sparseName, sparseFile)
	if err != nil {
		return nil, err
	}

	// Set the status of the loopback device
	loopInfo := &loopInfo64{
		loFileName: stringToLoopName(loopFile.Name()),
		loOffset:   0,
		loFlags:    LoFlagsAutoClear,
	}

	if err := ioctlLoopSetStatus64(loopFile.Fd(), loopInfo); err != nil {
		logrus.Errorf("Cannot set up loopback device info: %s", err)

		// If the call failed, then free the loopback device
		if err := ioctlLoopClrFd(loopFile.Fd()); err != nil {
			logrus.Error("While cleaning up the loopback device")
		}
		loopFile.Close()
		return nil, ErrAttachLoopbackDevice
	}

	return loopFile, nil
}
