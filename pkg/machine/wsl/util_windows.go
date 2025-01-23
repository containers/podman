//go:build windows

package wsl

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"

	"github.com/Microsoft/go-winio"
	"github.com/containers/storage/pkg/fileutils"
	"github.com/containers/storage/pkg/homedir"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

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

const (
	// ref: https://learn.microsoft.com/en-us/windows/win32/secauthz/privilege-constants#constants
	rebootPrivilege = "SeShutdownPrivilege"

	// "Application: Installation (Planned)" A planned restart or shutdown to perform application installation.
	// ref: https://learn.microsoft.com/en-us/windows/win32/shutdown/system-shutdown-reason-codes
	rebootReason = windows.SHTDN_REASON_MAJOR_APPLICATION | windows.SHTDN_REASON_MINOR_INSTALLATION | windows.SHTDN_REASON_FLAG_PLANNED

	// ref: https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-exitwindowsex#parameters
	rebootFlags = windows.EWX_REBOOT | windows.EWX_RESTARTAPPS | windows.EWX_FORCEIFHUNG
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

func HasAdminRights() bool {
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
	defer func() {
		_ = windows.FreeSid(sid)
	}()

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
	if err := os.WriteFile(commFile, []byte(encoded), 0600); err != nil {
		return fmt.Errorf("could not serialize command state: %w", err)
	}

	command := fmt.Sprintf(pShellLaunch, commFile)
	if err := fileutils.Lexists(filepath.Join(os.Getenv(localAppData), wtLocation)); err == nil {
		wtCommand := wtPrefix + command
		// RunOnce is limited to 260 chars (supposedly no longer in Builds >= 19489)
		// For now fallback in cases of long usernames (>89 chars)
		if len(wtCommand) < 260 {
			command = wtCommand
		}
	}

	message := "To continue the process of enabling WSL, the system needs to reboot. " +
		"Alternatively, you can cancel and reboot manually\n\n" +
		"After rebooting, please wait a minute or two for podman machine to relaunch and continue installing."

	if MessageBox(message, "Podman Machine", false) != 1 {
		fmt.Println("Reboot is required to continue installation, please reboot at your convenience")
		os.Exit(ErrorSuccessRebootRequired)
		return nil
	}

	if err := addRunOnceRegistryEntry(command); err != nil {
		return err
	}

	if err := winio.RunWithPrivilege(rebootPrivilege, func() error {
		if err := windows.ExitWindowsEx(rebootFlags, rebootReason); err != nil {
			return fmt.Errorf("execute ExitWindowsEx to reboot system failed: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("cannot reboot system: %w", err)
	}

	return nil
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
	buf := new(bytes.Buffer)
	for _, r := range u16 {
		_ = binary.Write(buf, binary.LittleEndian, r)
	}
	return buf.Bytes()
}

func MessageBox(caption, title string, fail bool) int {
	var format uint32
	if fail {
		format = windows.MB_ICONERROR
	} else {
		format = windows.MB_OKCANCEL | windows.MB_ICONINFORMATION
	}

	captionPtr, _ := syscall.UTF16PtrFromString(caption)
	titlePtr, _ := syscall.UTF16PtrFromString(title)

	ret, _ := windows.MessageBox(0, captionPtr, titlePtr, format)

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
