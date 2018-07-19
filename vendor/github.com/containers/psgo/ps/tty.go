package ps

import (
	"os"
	"strings"
	"syscall"
)

// tty represents a tty including its minor and major device number and the
// path to the tty.
type tty struct {
	// minor device number
	minor uint64
	// major device number
	major uint64
	// path to the tty
	device string
}

// majDevNum returns the major device number of rdev (see stat_t.Rdev).
func majDevNum(rdev uint64) uint64 {
	return (rdev >> 8) & 0xfff
}

// minDevNum returns the minor device number of rdev (see stat_t.Rdev).
func minDevNum(rdev uint64) uint64 {
	return (rdev & 0xff) | ((rdev >> 12) & 0xfff00)
}

// ttyNrToDev returns the major and minor number of the tty device from
// /proc/$pid/stat.tty_nr as described in man 5 proc.
func ttyNrToDev(ttyNr uint64) (uint64, uint64) {
	// (man 5 proc) The minor device number is contained in the combination
	// of bits 31 to 20 and 7 to 0; the major device number is in bits 15
	// to 8.
	maj := (ttyNr >> 8) & 0xFF
	min := (ttyNr & 0xFF) | ((ttyNr >> 20) & 0xFFF)
	return maj, min
}

// findTTY returns a tty with the corresponding major and minor device number
// or nil if no matching tty is found.
func findTTY(maj, min uint64) (*tty, error) {
	if len(ttyDevices) == 0 {
		var err error
		ttyDevices, err = getTTYs()
		if err != nil {
			return nil, err
		}
	}
	for _, t := range ttyDevices {
		if t.minor == min && t.major == maj {
			return t, nil
		}
	}
	return nil, nil
}

// getTTYs parses /dev for tty and pts devices.
func getTTYs() ([]*tty, error) {
	devDir, err := os.Open("/dev/")
	if err != nil {
		return nil, err
	}
	defer devDir.Close()

	devices := []string{}
	devTTYs, err := devDir.Readdirnames(0)
	if err != nil {
		return nil, err
	}
	for _, d := range devTTYs {
		if !strings.HasPrefix(d, "tty") {
			continue
		}
		devices = append(devices, "/dev/"+d)
	}

	devPTSDir, err := os.Open("/dev/pts/")
	if err != nil {
		return nil, err
	}
	defer devPTSDir.Close()

	devPTSs, err := devPTSDir.Readdirnames(0)
	if err != nil {
		return nil, err
	}
	for _, d := range devPTSs {
		devices = append(devices, "/dev/pts/"+d)
	}

	ttys := []*tty{}
	for _, dev := range devices {
		fi, err := os.Stat(dev)
		if err != nil {
			if os.IsNotExist(err) {
				// catch race conditions
				continue
			}
			return nil, err
		}
		s := fi.Sys().(*syscall.Stat_t)
		t := tty{
			minor:  minDevNum(s.Rdev),
			major:  majDevNum(s.Rdev),
			device: dev,
		}
		ttys = append(ttys, &t)
	}

	return ttys, nil
}
