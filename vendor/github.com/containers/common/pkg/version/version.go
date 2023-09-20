package version

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	UnknownPackage = "Unknown"
)

// Note: This function is copied from containers/podman libpod/util.go
// Please see https://github.com/containers/common/pull/1460
func queryPackageVersion(cmdArg ...string) string {
	output := UnknownPackage
	if 1 < len(cmdArg) {
		cmd := exec.Command(cmdArg[0], cmdArg[1:]...)
		if outp, err := cmd.Output(); err == nil {
			output = string(outp)
			deb := false
			if cmdArg[0] == "/usr/bin/dlocate" {
				// can return multiple matches
				l := strings.Split(output, "\n")
				output = l[0]
				deb = true
			} else if cmdArg[0] == "/usr/bin/dpkg" {
				deb = true
			}
			if deb {
				r := strings.Split(output, ": ")
				queryFormat := `${Package}_${Version}_${Architecture}`
				cmd = exec.Command("/usr/bin/dpkg-query", "-f", queryFormat, "-W", r[0])
				if outp, err := cmd.Output(); err == nil {
					output = string(outp)
				}
			}
		}
		if cmdArg[0] == "/sbin/apk" {
			prefix := cmdArg[len(cmdArg)-1] + " is owned by "
			output = strings.Replace(output, prefix, "", 1)
		}
	}
	return strings.Trim(output, "\n")
}

// Note: This function is copied from containers/podman libpod/util.go
// Please see https://github.com/containers/common/pull/1460
func Package(program string) string { // program is full path
	_, err := os.Stat(program)
	if err != nil {
		return UnknownPackage
	}
	packagers := [][]string{
		{"/usr/bin/rpm", "-q", "-f"},
		{"/usr/bin/dlocate", "-F"},             // Debian, Ubuntu (quick)
		{"/usr/bin/dpkg", "-S"},                // Debian, Ubuntu (slow)
		{"/usr/bin/pacman", "-Qo"},             // Arch
		{"/usr/bin/qfile", "-qv"},              // Gentoo (quick)
		{"/usr/bin/equery", "b"},               // Gentoo (slow)
		{"/sbin/apk", "info", "-W"},            // Alpine
		{"/usr/local/sbin/pkg", "which", "-q"}, // FreeBSD
	}

	for _, cmd := range packagers {
		cmd = append(cmd, program)
		if out := queryPackageVersion(cmd...); out != UnknownPackage {
			return out
		}
	}
	return UnknownPackage
}

// Note: This function is copied from containers/podman libpod/util.go
// Please see https://github.com/containers/common/pull/1460
func Program(name string) (string, error) {
	return program(name, false)
}

func ProgramDnsname(name string) (string, error) {
	return program(name, true)
}

func program(program string, dnsname bool) (string, error) {
	cmd := exec.Command(program, "--version")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("`%v --version` failed: %v %v (%v)", program, stderr.String(), stdout.String(), err)
	}

	output := strings.TrimSuffix(stdout.String(), "\n")
	// dnsname --version returns the information to stderr
	if dnsname {
		output = strings.TrimSuffix(stderr.String(), "\n")
	}

	return output, nil
}
