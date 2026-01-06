//go:build windows

package powershell

import (
	"errors"
	"strings"
)

var (
	ErrPowerShellNotFound = errors.New("powershell was not found in the path")
	ErrNotAdministrator   = errors.New("hyper-v commands have to be run as an administrator")
	ErrNotInstalled       = errors.New("hyper-v powershell module is not available")
)

func cmdOut(args ...string) (string, error) {
	stdout, _, err := Execute(args...)
	return stdout, err
}

func HypervAvailable() error {
	stdout, err := cmdOut("@(Get-Module -ListAvailable hyper-v).Name | Get-Unique")
	if err != nil {
		return err
	}

	resp := firstLine(stdout)

	if resp != "Hyper-V" {
		return ErrNotInstalled
	}

	return nil
}

func IsHypervAdministrator() bool {
	stdout, err := cmdOut(`@([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole(([System.Security.Principal.SecurityIdentifier]::new("S-1-5-32-578")))`)
	if err != nil {
		return false
	}

	resp := firstLine(stdout)
	return resp == "True"
}

func firstLine(stdout string) string {
	stdout = strings.TrimSpace(stdout)
	if len(strings.Split(stdout, "\n")) == 0 {
		return stdout
	}
	return strings.Split(stdout, "\n")[0]
}
