package wsl

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/containers/storage/pkg/homedir"
)

//nolint
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

//nolint
type Luid struct {
	lowPart  uint32
	highPart int32
}

type LuidAndAttributes struct {
	luid       Luid
	attributes uint32
}

type TokenPrivileges struct {
	privilegeCount uint32
	privileges     [1]LuidAndAttributes
}

//nolint // Cleaner to refer to the official OS constant names, and consistent with syscall
const (
	SEE_MASK_NOCLOSEPROCESS         = 0x40
	EWX_FORCEIFHUNG                 = 0x10
	EWX_REBOOT                      = 0x02
	EWX_RESTARTAPPS                 = 0x40
	SHTDN_REASON_MAJOR_APPLICATION  = 0x00040000
	SHTDN_REASON_MINOR_INSTALLATION = 0x00000002
	SHTDN_REASON_FLAG_PLANNED       = 0x80000000
	TOKEN_ADJUST_PRIVILEGES         = 0x0020
	TOKEN_QUERY                     = 0x0008
	SE_PRIVILEGE_ENABLED            = 0x00000002
	SE_ERR_ACCESSDENIED             = 0x05
	WM_QUIT                         = 0x12
)

func winVersionAtLeast(major uint, minor uint, build uint) bool {
	var out [3]uint32

	in := []uint32{uint32(major), uint32(minor), uint32(build)}
	out[0], out[1], out[2] = windows.RtlGetNtVersionNumbers()

	for i, o := range out {
		if in[i] > o {
			return false
		}
		if in[i] < o {
			return true
		}
	}

	return true
}

func hasAdminRights() bool {
	var sid *windows.SID

	// See: https://coolaj86.com/articles/golang-and-windows-and-admins-oh-my/
	if err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid); err != nil {
		logrus.Warnf("SID allocation error: %s", err)
		return false
	}
	defer windows.FreeSid(sid)

	//  From MS docs:
	// "If TokenHandle is NULL, CheckTokenMembership uses the impersonation
	//  token of the calling thread. If the thread is not impersonating,
	//  the function duplicates the thread's primary token to create an
	//  impersonation token."
	token := windows.Token(0)

	member, err := token.IsMember(sid)
	if err != nil {
		logrus.Warnf("Token Membership Error: %s", err)
		return false
	}

	return member || token.IsElevated()
}

func relaunchElevatedWait() error {
	e, _ := os.Executable()
	d, _ := os.Getwd()
	exe, _ := syscall.UTF16PtrFromString(e)
	cwd, _ := syscall.UTF16PtrFromString(d)
	arg, _ := syscall.UTF16PtrFromString(buildCommandArgs(true))
	verb, _ := syscall.UTF16PtrFromString("runas")

	shell32 := syscall.NewLazyDLL("shell32.dll")

	info := &SHELLEXECUTEINFO{
		fMask:        SEE_MASK_NOCLOSEPROCESS,
		hwnd:         0,
		lpVerb:       uintptr(unsafe.Pointer(verb)),
		lpFile:       uintptr(unsafe.Pointer(exe)),
		lpParameters: uintptr(unsafe.Pointer(arg)),
		lpDirectory:  uintptr(unsafe.Pointer(cwd)),
		nShow:        1,
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

	handle := syscall.Handle(info.hProcess)
	defer syscall.CloseHandle(handle)

	w, err := syscall.WaitForSingleObject(handle, syscall.INFINITE)
	switch w {
	case syscall.WAIT_OBJECT_0:
		break
	case syscall.WAIT_FAILED:
		return fmt.Errorf("could not wait for process, failed: %w", err)
	default:
		return errors.New("could not wait for process, unknown error")
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

func reboot() error {
	const (
		wtLocation   = `Microsoft\WindowsApps\wt.exe`
		wtPrefix     = `%LocalAppData%\Microsoft\WindowsApps\wt -p "Windows PowerShell" `
		localAppData = "LocalAppData"
		pShellLaunch = `powershell -noexit "powershell -EncodedCommand (Get-Content '%s')"`
	)

	exe, _ := os.Executable()
	relaunch := fmt.Sprintf("& %s %s", syscall.EscapeArg(exe), buildCommandArgs(false))
	encoded := base64.StdEncoding.EncodeToString(encodeUTF16Bytes(relaunch))

	dataDir, err := homedir.GetDataHome()
	if err != nil {
		return fmt.Errorf("could not determine data directory: %w", err)
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("could not create data directory: %w", err)
	}
	commFile := filepath.Join(dataDir, "podman-relaunch.dat")
	if err := ioutil.WriteFile(commFile, []byte(encoded), 0600); err != nil {
		return fmt.Errorf("could not serialize command state: %w", err)
	}

	command := fmt.Sprintf(pShellLaunch, commFile)
	if _, err := os.Lstat(filepath.Join(os.Getenv(localAppData), wtLocation)); err == nil {
		wtCommand := wtPrefix + command
		// RunOnce is limited to 260 chars (supposedly no longer in Builds >= 19489)
		// For now fallbacak in cases of long usernames (>89 chars)
		if len(wtCommand) < 260 {
			command = wtCommand
		}
	}

	if err := addRunOnceRegistryEntry(command); err != nil {
		return err
	}

	if err := obtainShutdownPrivilege(); err != nil {
		return err
	}

	message := "To continue the process of enabling WSL, the system needs to reboot. " +
		"Alternatively, you can cancel and reboot manually\n\n" +
		"After rebooting, please wait a minute or two for podman machine to relaunch and continue installing."

	if MessageBox(message, "Podman Machine", false) != 1 {
		fmt.Println("Reboot is required to continue installation, please reboot at your convenience")
		os.Exit(ErrorSuccessRebootRequired)
		return nil
	}

	user32 := syscall.NewLazyDLL("user32")
	procExit := user32.NewProc("ExitWindowsEx")
	if ret, _, err := procExit.Call(EWX_REBOOT|EWX_RESTARTAPPS|EWX_FORCEIFHUNG,
		SHTDN_REASON_MAJOR_APPLICATION|SHTDN_REASON_MINOR_INSTALLATION|SHTDN_REASON_FLAG_PLANNED); ret != 1 {
		return fmt.Errorf("reboot failed: %w", err)
	}

	return nil
}

func obtainShutdownPrivilege() error {
	const SeShutdownName = "SeShutdownPrivilege"

	advapi32 := syscall.NewLazyDLL("advapi32")
	OpenProcessToken := advapi32.NewProc("OpenProcessToken")
	LookupPrivilegeValue := advapi32.NewProc("LookupPrivilegeValueW")
	AdjustTokenPrivileges := advapi32.NewProc("AdjustTokenPrivileges")

	proc, _ := syscall.GetCurrentProcess()

	var hToken uintptr
	if ret, _, err := OpenProcessToken.Call(uintptr(proc), TOKEN_ADJUST_PRIVILEGES|TOKEN_QUERY, uintptr(unsafe.Pointer(&hToken))); ret != 1 {
		return fmt.Errorf("error opening process token: %w", err)
	}

	var privs TokenPrivileges
	if ret, _, err := LookupPrivilegeValue.Call(uintptr(0), uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(SeShutdownName))), uintptr(unsafe.Pointer(&(privs.privileges[0].luid)))); ret != 1 {
		return fmt.Errorf("error looking up shutdown privilege: %w", err)
	}

	privs.privilegeCount = 1
	privs.privileges[0].attributes = SE_PRIVILEGE_ENABLED

	if ret, _, err := AdjustTokenPrivileges.Call(hToken, 0, uintptr(unsafe.Pointer(&privs)), 0, uintptr(0), 0); ret != 1 {
		return fmt.Errorf("error enabling shutdown privilege on token: %w", err)
	}

	return nil
}

func getProcessState(pid int) (active bool, exitCode int) {
	const da = syscall.STANDARD_RIGHTS_READ | syscall.PROCESS_QUERY_INFORMATION | syscall.SYNCHRONIZE
	handle, err := syscall.OpenProcess(da, false, uint32(pid))
	if err != nil {
		return false, int(syscall.ERROR_PROC_NOT_FOUND)
	}

	var code uint32
	syscall.GetExitCodeProcess(handle, &code)
	return code == 259, int(code)
}

func addRunOnceRegistryEntry(command string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\RunOnce`, registry.WRITE)
	if err != nil {
		return fmt.Errorf("could not open RunOnce registry entry: %w", err)
	}

	defer k.Close()

	if err := k.SetExpandStringValue("podman-machine", command); err != nil {
		return fmt.Errorf("could not open RunOnce registry entry: %w", err)
	}

	return nil
}

func encodeUTF16Bytes(s string) []byte {
	u16 := utf16.Encode([]rune(s))
	u16le := make([]byte, len(u16)*2)
	for i := 0; i < len(u16); i++ {
		u16le[i<<1] = byte(u16[i])
		u16le[(i<<1)+1] = byte(u16[i] >> 8)
	}
	return u16le
}

func MessageBox(caption, title string, fail bool) int {
	var format int
	if fail {
		format = 0x10
	} else {
		format = 0x41
	}

	user32 := syscall.NewLazyDLL("user32.dll")
	captionPtr, _ := syscall.UTF16PtrFromString(caption)
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	ret, _, _ := user32.NewProc("MessageBoxW").Call(
		uintptr(0),
		uintptr(unsafe.Pointer(captionPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(format))

	return int(ret)
}

func buildCommandArgs(elevate bool) string {
	var args []string
	for _, arg := range os.Args[1:] {
		if arg != "--reexec" {
			args = append(args, syscall.EscapeArg(arg))
			if elevate && arg == "init" {
				args = append(args, "--reexec")
			}
		}
	}
	return strings.Join(args, " ")
}

func sendQuit(tid uint32) {
	user32 := syscall.NewLazyDLL("user32.dll")
	postMessage := user32.NewProc("PostThreadMessageW")
	postMessage.Call(uintptr(tid), WM_QUIT, 0, 0)
}
