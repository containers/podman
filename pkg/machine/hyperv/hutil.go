//go:build windows

package hyperv

import (
	"errors"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

var (
	ErrHypervUserNotInAdminGroup             = errors.New("Hyper-V machines require Hyper-V admin rights to be managed. Please add the current user to the Hyper-V Administrators group or run Podman as an administrator")
	ErrHypervRegistryInitRequiresElevation   = errors.New("the first time Podman initializes a Hyper-V machine, it requires admin rights. Please run Podman as an administrator")
	ErrHypervRegistryRemoveRequiresElevation = errors.New("removing the last Hyper-V machine requires admin rights to clean up the Windows Registry. Please run Podman as an administrator")
	ErrHypervRegistryUpdateRequiresElevation = errors.New("this machine's configuration requires additional Hyper-V networking (hvsock) entries in the Windows Registry. Please run Podman as an administrator")
)

func HasHyperVAdminRights() bool {
	sid, err := windows.CreateWellKnownSid(windows.WinBuiltinHyperVAdminsSid)
	if err != nil {
		return false
	}

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

	return member
}
