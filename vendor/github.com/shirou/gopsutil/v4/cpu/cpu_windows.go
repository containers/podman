// SPDX-License-Identifier: BSD-3-Clause
//go:build windows

package cpu

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"unsafe"

	"github.com/yusufpapurcu/wmi"
	"golang.org/x/sys/windows"

	"github.com/shirou/gopsutil/v4/internal/common"
)

var (
	procGetNativeSystemInfo              = common.Modkernel32.NewProc("GetNativeSystemInfo")
	procGetLogicalProcessorInformationEx = common.Modkernel32.NewProc("GetLogicalProcessorInformationEx")
)

type win32_Processor struct { //nolint:revive //FIXME
	Family                    uint16
	Manufacturer              string
	Name                      string
	NumberOfLogicalProcessors uint32
	NumberOfCores             uint32
	ProcessorID               *string
	Stepping                  *string
	MaxClockSpeed             uint32
}

// SYSTEM_PROCESSOR_PERFORMANCE_INFORMATION
// defined in windows api doc with the following
// https://docs.microsoft.com/en-us/windows/desktop/api/winternl/nf-winternl-ntquerysysteminformation#system_processor_performance_information
// additional fields documented here
// https://www.geoffchappell.com/studies/windows/km/ntoskrnl/api/ex/sysinfo/processor_performance.htm
type win32_SystemProcessorPerformanceInformation struct { //nolint:revive //FIXME
	IdleTime       int64  // idle time in 100ns (this is not a filetime).
	KernelTime     int64  // kernel time in 100ns.  kernel time includes idle time. (this is not a filetime).
	UserTime       int64  // usertime in 100ns (this is not a filetime).
	DpcTime        int64  // dpc time in 100ns (this is not a filetime).
	InterruptTime  int64  // interrupt time in 100ns
	InterruptCount uint64 // ULONG needs to be uint64
}

const (
	ClocksPerSec = 10000000.0

	// systemProcessorPerformanceInformationClass information class to query with NTQuerySystemInformation
	// https://processhacker.sourceforge.io/doc/ntexapi_8h.html#ad5d815b48e8f4da1ef2eb7a2f18a54e0
	win32_SystemProcessorPerformanceInformationClass = 8 //nolint:revive //FIXME

	// size of systemProcessorPerformanceInfoSize in memory
	win32_SystemProcessorPerformanceInfoSize = uint32(unsafe.Sizeof(win32_SystemProcessorPerformanceInformation{})) //nolint:revive //FIXME
)

// Times returns times stat per cpu and combined for all CPUs
func Times(percpu bool) ([]TimesStat, error) {
	return TimesWithContext(context.Background(), percpu)
}

func TimesWithContext(_ context.Context, percpu bool) ([]TimesStat, error) {
	if percpu {
		return perCPUTimes()
	}

	var ret []TimesStat
	var lpIdleTime common.FILETIME
	var lpKernelTime common.FILETIME
	var lpUserTime common.FILETIME
	// GetSystemTimes returns 0 for error, in which case we check err,
	// see https://pkg.go.dev/golang.org/x/sys/windows#LazyProc.Call
	r, _, err := common.ProcGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&lpIdleTime)),
		uintptr(unsafe.Pointer(&lpKernelTime)),
		uintptr(unsafe.Pointer(&lpUserTime)))
	if r == 0 {
		return nil, err
	}

	LOT := float64(0.0000001)
	HIT := (LOT * 4294967296.0)
	idle := ((HIT * float64(lpIdleTime.DwHighDateTime)) + (LOT * float64(lpIdleTime.DwLowDateTime)))
	user := ((HIT * float64(lpUserTime.DwHighDateTime)) + (LOT * float64(lpUserTime.DwLowDateTime)))
	kernel := ((HIT * float64(lpKernelTime.DwHighDateTime)) + (LOT * float64(lpKernelTime.DwLowDateTime)))
	system := (kernel - idle)

	ret = append(ret, TimesStat{
		CPU:    "cpu-total",
		Idle:   float64(idle),
		User:   float64(user),
		System: float64(system),
	})
	return ret, nil
}

func Info() ([]InfoStat, error) {
	return InfoWithContext(context.Background())
}

func InfoWithContext(ctx context.Context) ([]InfoStat, error) {
	var ret []InfoStat
	var dst []win32_Processor
	q := wmi.CreateQuery(&dst, "")
	if err := common.WMIQueryWithContext(ctx, q, &dst); err != nil {
		return ret, err
	}

	var procID string
	for i, l := range dst {
		procID = ""
		if l.ProcessorID != nil {
			procID = *l.ProcessorID
		}

		cpu := InfoStat{
			CPU:        int32(i),
			Family:     strconv.FormatUint(uint64(l.Family), 10),
			VendorID:   l.Manufacturer,
			ModelName:  l.Name,
			Cores:      int32(l.NumberOfLogicalProcessors),
			PhysicalID: procID,
			Mhz:        float64(l.MaxClockSpeed),
			Flags:      []string{},
		}
		ret = append(ret, cpu)
	}

	return ret, nil
}

// perCPUTimes returns times stat per cpu, per core and overall for all CPUs
func perCPUTimes() ([]TimesStat, error) {
	var ret []TimesStat
	stats, err := perfInfo()
	if err != nil {
		return nil, err
	}
	for core, v := range stats {
		c := TimesStat{
			CPU:    fmt.Sprintf("cpu%d", core),
			User:   float64(v.UserTime) / ClocksPerSec,
			System: float64(v.KernelTime-v.IdleTime) / ClocksPerSec,
			Idle:   float64(v.IdleTime) / ClocksPerSec,
			Irq:    float64(v.InterruptTime) / ClocksPerSec,
		}
		ret = append(ret, c)
	}
	return ret, nil
}

// makes call to Windows API function to retrieve performance information for each core
func perfInfo() ([]win32_SystemProcessorPerformanceInformation, error) {
	// Make maxResults large for safety.
	// We can't invoke the api call with a results array that's too small.
	// If we have more than 2056 cores on a single host, then it's probably the future.
	maxBuffer := 2056
	// buffer for results from the windows proc
	resultBuffer := make([]win32_SystemProcessorPerformanceInformation, maxBuffer)
	// size of the buffer in memory
	bufferSize := uintptr(win32_SystemProcessorPerformanceInfoSize) * uintptr(maxBuffer)
	// size of the returned response
	var retSize uint32

	// Invoke windows api proc.
	// The returned err from the windows dll proc will always be non-nil even when successful.
	// See https://godoc.org/golang.org/x/sys/windows#LazyProc.Call for more information
	retCode, _, err := common.ProcNtQuerySystemInformation.Call(
		win32_SystemProcessorPerformanceInformationClass, // System Information Class -> SystemProcessorPerformanceInformation
		uintptr(unsafe.Pointer(&resultBuffer[0])),        // pointer to first element in result buffer
		bufferSize,                        // size of the buffer in memory
		uintptr(unsafe.Pointer(&retSize)), // pointer to the size of the returned results the windows proc will set this
	)

	// check return code for errors
	if retCode != 0 {
		return nil, fmt.Errorf("call to NtQuerySystemInformation returned %d. err: %s", retCode, err.Error())
	}

	// calculate the number of returned elements based on the returned size
	numReturnedElements := retSize / win32_SystemProcessorPerformanceInfoSize

	// trim results to the number of returned elements
	resultBuffer = resultBuffer[:numReturnedElements]

	return resultBuffer, nil
}

// SystemInfo is an equivalent representation of SYSTEM_INFO in the Windows API.
// https://msdn.microsoft.com/en-us/library/ms724958%28VS.85%29.aspx?f=255&MSPPError=-2147217396
// https://github.com/elastic/go-windows/blob/bb1581babc04d5cb29a2bfa7a9ac6781c730c8dd/kernel32.go#L43
type systemInfo struct {
	wProcessorArchitecture      uint16
	wReserved                   uint16
	dwPageSize                  uint32
	lpMinimumApplicationAddress uintptr
	lpMaximumApplicationAddress uintptr
	dwActiveProcessorMask       uintptr
	dwNumberOfProcessors        uint32
	dwProcessorType             uint32
	dwAllocationGranularity     uint32
	wProcessorLevel             uint16
	wProcessorRevision          uint16
}

type groupAffinity struct {
	mask     uintptr // https://learn.microsoft.com/it-it/windows-hardware/drivers/kernel/interrupt-affinity-and-priority#about-kaffinity
	group    uint16
	reserved [3]uint16
}

// https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-processor_relationship
type processorRelationship struct {
	flags          byte
	efficientClass byte
	reserved       [20]byte
	groupCount     uint16
	groupMask      [1]groupAffinity
}

// https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-system_logical_processor_information_ex
type systemLogicalProcessorInformationEx struct {
	Relationship uint32
	Size         uint32
	Processor    processorRelationship
}

func getPhysicalCoreCount() (int, error) {
	var length uint32
	const relationAll = 0xffff
	const relationProcessorCore = 0x0

	// First call to determine the required buffer size
	_, _, err := procGetLogicalProcessorInformationEx.Call(uintptr(relationAll), 0, uintptr(unsafe.Pointer(&length)))
	if err != nil && !errors.Is(err, windows.ERROR_INSUFFICIENT_BUFFER) {
		return 0, fmt.Errorf("failed to get buffer size: %w", err)
	}

	// Allocate the buffer
	buffer := make([]byte, length)

	// Second call to retrieve the processor information
	_, _, err = procGetLogicalProcessorInformationEx.Call(uintptr(relationAll), uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&length)))
	if err != nil && !errors.Is(err, windows.NTE_OP_OK) {
		return 0, fmt.Errorf("failed to get logical processor information: %w", err)
	}

	// Iterate through the buffer to count physical cores
	offset := uintptr(0)
	ncpus := 0
	for offset < uintptr(length) {
		info := (*systemLogicalProcessorInformationEx)(unsafe.Pointer(uintptr(unsafe.Pointer(&buffer[0])) + offset))
		if info.Relationship == relationProcessorCore {
			ncpus++
		}
		offset += uintptr(info.Size)
	}

	return ncpus, nil
}

func CountsWithContext(_ context.Context, logical bool) (int, error) {
	if logical {
		// Get logical processor count https://github.com/giampaolo/psutil/blob/d01a9eaa35a8aadf6c519839e987a49d8be2d891/psutil/_psutil_windows.c#L97
		ret := windows.GetActiveProcessorCount(windows.ALL_PROCESSOR_GROUPS)
		if ret != 0 {
			return int(ret), nil
		}

		var systemInfo systemInfo
		_, _, err := procGetNativeSystemInfo.Call(uintptr(unsafe.Pointer(&systemInfo)))
		if systemInfo.dwNumberOfProcessors == 0 {
			return 0, err
		}
		return int(systemInfo.dwNumberOfProcessors), nil
	}

	// Get physical core count https://github.com/giampaolo/psutil/blob/d01a9eaa35a8aadf6c519839e987a49d8be2d891/psutil/_psutil_windows.c#L499
	return getPhysicalCoreCount()
}
