// SPDX-License-Identifier: BSD-3-Clause
//go:build windows

package mem

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// ExVirtualMemory represents Windows specific information
// https://learn.microsoft.com/en-us/windows/win32/api/sysinfoapi/ns-sysinfoapi-memorystatusex
type ExVirtualMemory struct {
	VirtualTotal uint64 `json:"virtualTotal"`
	VirtualAvail uint64 `json:"virtualAvail"`
}

type ExWindows struct{}

func NewExWindows() *ExWindows {
	return &ExWindows{}
}

func (e *ExWindows) VirtualMemory() (*ExVirtualMemory, error) {
	var memInfo memoryStatusEx
	memInfo.cbSize = uint32(unsafe.Sizeof(memInfo))
	mem, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&memInfo)))
	if mem == 0 {
		return nil, windows.GetLastError()
	}

	ret := &ExVirtualMemory{
		VirtualTotal: memInfo.ullTotalVirtual,
		VirtualAvail: memInfo.ullAvailVirtual,
	}

	return ret, nil
}
