//go:build windows

package windows

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

type SHELLEXECUTEINFO struct {
	cbSize         uint32
	fMask          uint32
	hwnd           syscall.Handle
	lpVerb         uintptr
	lpFile         uintptr
	lpParameters   uintptr
	lpDirectory    uintptr
	nShow          int
	hInstApp       syscall.Handle
	lpIDList       uintptr
	lpClass        uintptr
	hkeyClass      syscall.Handle
	dwHotKey       uint32
	hIconOrMonitor syscall.Handle
	hProcess       syscall.Handle
}

// Cleaner to refer to the official OS constant names, and consistent with syscall
// Ref: https://learn.microsoft.com/en-us/windows/win32/api/shellapi/ns-shellapi-shellexecuteinfow#members
const (
	//nolint:stylecheck
	SEE_MASK_NOCLOSEPROCESS = 0x40
	//nolint:stylecheck
	SE_ERR_ACCESSDENIED = 0x05
)

type ExitCodeError struct {
	Code uint
}

func (e *ExitCodeError) Error() string {
	return fmt.Sprintf("Process failed with exit code: %d", e.Code)
}

func LaunchElevatedWait(exe string, cwd string, args string) error {
	return LaunchElevatedWaitWithWindowMode(exe, cwd, args, syscall.SW_SHOWNORMAL)
}

func LaunchElevatedWaitWithWindowMode(exe string, cwd string, args string, windowMode int) error {
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	cwdPtr, _ := syscall.UTF16PtrFromString(cwd)
	arg, _ := syscall.UTF16PtrFromString(args)
	verb, _ := syscall.UTF16PtrFromString("runas")

	shell32 := syscall.NewLazyDLL("shell32.dll")

	info := &SHELLEXECUTEINFO{
		fMask:        SEE_MASK_NOCLOSEPROCESS,
		hwnd:         0,
		lpVerb:       uintptr(unsafe.Pointer(verb)),
		lpFile:       uintptr(unsafe.Pointer(exePtr)),
		lpParameters: uintptr(unsafe.Pointer(arg)),
		lpDirectory:  uintptr(unsafe.Pointer(cwdPtr)),
		nShow:        windowMode,
	}
	info.cbSize = uint32(unsafe.Sizeof(*info))
	procShellExecuteEx := shell32.NewProc("ShellExecuteExW")
	if ret, _, _ := procShellExecuteEx.Call(uintptr(unsafe.Pointer(info))); ret == 0 { // 0 = False
		err := syscall.GetLastError()
		if info.hInstApp == SE_ERR_ACCESSDENIED {
			return wrapMaybe(err, "request to elevate privileges was denied")
		}
		return wrapMaybef(err, "could not launch process, ShellEX Error = %d", info.hInstApp)
	}

	handle := info.hProcess
	defer func() {
		_ = syscall.CloseHandle(handle)
	}()

	w, err := syscall.WaitForSingleObject(handle, syscall.INFINITE)
	switch w {
	case syscall.WAIT_OBJECT_0:
		break
	case syscall.WAIT_FAILED:
		return fmt.Errorf("could not wait for process, failed: %w", err)
	default:
		return fmt.Errorf("could not wait for process, unknown error. event: %X, err: %v", w, err)
	}
	var code uint32
	if err := syscall.GetExitCodeProcess(handle, &code); err != nil {
		return err
	}
	if code != 0 {
		return &ExitCodeError{uint(code)}
	}

	return nil
}

func wrapMaybe(err error, message string) error {
	if err != nil {
		return fmt.Errorf("%v: %w", message, err)
	}

	return errors.New(message)
}

func wrapMaybef(err error, format string, args ...interface{}) error {
	if err != nil {
		return fmt.Errorf(format+": %w", append(args, err)...)
	}

	return fmt.Errorf(format, args...)
}
