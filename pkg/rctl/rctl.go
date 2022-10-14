//go:build freebsd
// +build freebsd

package rctl

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/sirupsen/logrus"
)

func GetRacct(filter string) (map[string]uint64, error) {
	bp, err := syscall.ByteSliceFromString(filter)
	if err != nil {
		return nil, err
	}
	var buf [1024]byte
	_, _, errno := syscall.Syscall6(syscall.SYS_RCTL_GET_RACCT,
		uintptr(unsafe.Pointer(&bp[0])),
		uintptr(len(bp)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)), 0, 0)
	if errno != 0 {
		return nil, fmt.Errorf("error calling rctl_get_racct with filter %s: %v", errno)
	}
	len := bytes.IndexByte(buf[:], byte(0))
	entries := strings.Split(string(buf[:len]), ",")
	res := make(map[string]uint64)
	for _, entry := range entries {
		kv := strings.SplitN(entry, "=", 2)
		key := kv[0]
		val, err := strconv.ParseUint(kv[1], 10, 0)
		if err != nil {
			logrus.Warnf("unexpected rctl entry, ignoring: %s", entry)
		}
		res[key] = val
	}
	return res, nil
}
