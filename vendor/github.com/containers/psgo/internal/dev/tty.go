package dev

import (
	"os"
	"strings"
	"syscall"
)

// TTY represents a tty including its minor and major device number and the
// path to the tty.
type TTY struct {
	// Minor device number.
	Minor uint64
	// Major device number.
	Major uint64
	// Path to the tty device.
	Path string
}

// cache TTYs to avoid redundant lookups
var devices *[]TTY

// FindTTY return the corresponding TTY to the ttyNr or nil of non could be
// found.
func FindTTY(ttyNr uint64) (*TTY, error) {
	// (man 5 proc) The minor device number is contained in the combination
	// of bits 31 to 20 and 7 to 0; the major device number is in bits 15
	// to 8.
	maj := (ttyNr >> 8) & 0xFF
	min := (ttyNr & 0xFF) | ((ttyNr >> 20) & 0xFFF)

	if devices == nil {
		devs, err := getTTYs()
		if err != nil {
			return nil, err
		}
		devices = devs
	}

	for _, t := range *devices {
		if t.Minor == min && t.Major == maj {
			return &t, nil
		}
	}

	return nil, nil
}

// majDevNum returns the major device number of rdev (see stat_t.Rdev).
func majDevNum(rdev uint64) uint64 {
	return (rdev >> 8) & 0xfff
}

// minDevNum returns the minor device number of rdev (see stat_t.Rdev).
func minDevNum(rdev uint64) uint64 {
	return (rdev & 0xff) | ((rdev >> 12) & 0xfff00)
}

// getTTYs parses /dev for tty and pts devices.
func getTTYs() (*[]TTY, error) {
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

	ttys := []TTY{}
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
		t := TTY{
			Minor: minDevNum(s.Rdev),
			Major: majDevNum(s.Rdev),
			Path:  dev,
		}
		ttys = append(ttys, t)
	}

	return &ttys, nil
}
