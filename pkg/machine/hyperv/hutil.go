//go:build windows

package hyperv

import (
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
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
