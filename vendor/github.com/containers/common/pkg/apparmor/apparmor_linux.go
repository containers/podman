// +build linux,apparmor

package apparmor

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"text/template"

	"github.com/containers/common/pkg/apparmor/internal/supported"
	"github.com/containers/storage/pkg/unshare"
	runcaa "github.com/opencontainers/runc/libcontainer/apparmor"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// profileDirectory is the file store for apparmor profiles and macros.
var profileDirectory = "/etc/apparmor.d"

// IsEnabled returns true if AppArmor is enabled on the host. It also checks
// for the existence of the `apparmor_parser` binary, which will be required to
// apply profiles.
func IsEnabled() bool {
	return supported.NewAppArmorVerifier().IsSupported() == nil
}

// profileData holds information about the given profile for generation.
type profileData struct {
	// Name is profile name.
	Name string
	// Imports defines the apparmor functions to import, before defining the profile.
	Imports []string
	// InnerImports defines the apparmor functions to import in the profile.
	InnerImports []string
	// Version is the {major, minor, patch} version of apparmor_parser as a single number.
	Version int
}

// generateDefault creates an apparmor profile from ProfileData.
func (p *profileData) generateDefault(apparmorParserPath string, out io.Writer) error {
	compiled, err := template.New("apparmor_profile").Parse(defaultProfileTemplate)
	if err != nil {
		return errors.Wrap(err, "create AppArmor profile from template")
	}

	if macroExists("tunables/global") {
		p.Imports = append(p.Imports, "#include <tunables/global>")
	} else {
		p.Imports = append(p.Imports, "@{PROC}=/proc/")
	}

	if macroExists("abstractions/base") {
		p.InnerImports = append(p.InnerImports, "#include <abstractions/base>")
	}

	ver, err := getAAParserVersion(apparmorParserPath)
	if err != nil {
		return errors.Wrap(err, "get AppArmor version")
	}
	p.Version = ver

	return errors.Wrap(compiled.Execute(out, p), "execute compiled profile")
}

// macrosExists checks if the passed macro exists.
func macroExists(m string) bool {
	_, err := os.Stat(path.Join(profileDirectory, m))
	return err == nil
}

// InstallDefault generates a default profile and loads it into the kernel
// using 'apparmor_parser'.
func InstallDefault(name string) error {
	if unshare.IsRootless() {
		return ErrApparmorRootless
	}

	p := profileData{
		Name: name,
	}

	apparmorParserPath, err := supported.NewAppArmorVerifier().FindAppArmorParserBinary()
	if err != nil {
		return errors.Wrap(err, "find `apparmor_parser` binary")
	}

	cmd := exec.Command(apparmorParserPath, "-Kr")
	pipe, err := cmd.StdinPipe()
	if err != nil {
		return errors.Wrapf(err, "execute %s", apparmorParserPath)
	}
	if err := cmd.Start(); err != nil {
		if pipeErr := pipe.Close(); pipeErr != nil {
			logrus.Errorf("unable to close AppArmor pipe: %q", pipeErr)
		}
		return errors.Wrapf(err, "start %s command", apparmorParserPath)
	}
	if err := p.generateDefault(apparmorParserPath, pipe); err != nil {
		if pipeErr := pipe.Close(); pipeErr != nil {
			logrus.Errorf("unable to close AppArmor pipe: %q", pipeErr)
		}
		if cmdErr := cmd.Wait(); cmdErr != nil {
			logrus.Errorf("unable to wait for AppArmor command: %q", cmdErr)
		}
		return errors.Wrap(err, "generate default profile into pipe")
	}

	if pipeErr := pipe.Close(); pipeErr != nil {
		logrus.Errorf("unable to close AppArmor pipe: %q", pipeErr)
	}

	return errors.Wrap(cmd.Wait(), "wait for AppArmor command")
}

// DefaultContent returns the default profile content as byte slice. The
// profile is named as the provided `name`. The function errors if the profile
// generation fails.
func DefaultContent(name string) ([]byte, error) {
	p := profileData{Name: name}
	buffer := &bytes.Buffer{}

	apparmorParserPath, err := supported.NewAppArmorVerifier().FindAppArmorParserBinary()
	if err != nil {
		return nil, errors.Wrap(err, "find `apparmor_parser` binary")
	}

	if err := p.generateDefault(apparmorParserPath, buffer); err != nil {
		return nil, errors.Wrap(err, "generate default AppAmor profile")
	}
	return buffer.Bytes(), nil
}

// IsLoaded checks if a profile with the given name has been loaded into the
// kernel.
func IsLoaded(name string) (bool, error) {
	if name != "" && unshare.IsRootless() {
		return false, errors.Wrapf(ErrApparmorRootless, "cannot load AppArmor profile %q", name)
	}

	file, err := os.Open("/sys/kernel/security/apparmor/profiles")
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, errors.Wrap(err, "open AppArmor profile path")
	}
	defer file.Close()

	r := bufio.NewReader(file)
	for {
		p, err := r.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, errors.Wrap(err, "reading AppArmor profile")
		}
		if strings.HasPrefix(p, name+" ") {
			return true, nil
		}
	}

	return false, nil
}

// execAAParser runs `apparmor_parser` with the passed arguments.
func execAAParser(apparmorParserPath, dir string, args ...string) (string, error) {
	c := exec.Command(apparmorParserPath, args...)
	c.Dir = dir

	output, err := c.Output()
	if err != nil {
		return "", errors.Errorf("running `%s %s` failed with output: %s\nerror: %v", c.Path, strings.Join(c.Args, " "), output, err)
	}

	return string(output), nil
}

// getAAParserVersion returns the major and minor version of apparmor_parser.
func getAAParserVersion(apparmorParserPath string) (int, error) {
	output, err := execAAParser(apparmorParserPath, "", "--version")
	if err != nil {
		return -1, errors.Wrap(err, "execute apparmor_parser")
	}
	return parseAAParserVersion(output)
}

// parseAAParserVersion parses the given `apparmor_parser --version` output and
// returns the major and minor version number as an integer.
func parseAAParserVersion(output string) (int, error) {
	// output is in the form of the following:
	// AppArmor parser version 2.9.1
	// Copyright (C) 1999-2008 Novell Inc.
	// Copyright 2009-2012 Canonical Ltd.
	lines := strings.SplitN(output, "\n", 2)
	words := strings.Split(lines[0], " ")
	version := words[len(words)-1]

	// split by major minor version
	v := strings.Split(version, ".")
	if len(v) == 0 || len(v) > 3 {
		return -1, errors.Errorf("parsing version failed for output: `%s`", output)
	}

	// Default the versions to 0.
	var majorVersion, minorVersion, patchLevel int

	majorVersion, err := strconv.Atoi(v[0])
	if err != nil {
		return -1, errors.Wrap(err, "convert AppArmor major version")
	}

	if len(v) > 1 {
		minorVersion, err = strconv.Atoi(v[1])
		if err != nil {
			return -1, errors.Wrap(err, "convert AppArmor minor version")
		}
	}
	if len(v) > 2 {
		patchLevel, err = strconv.Atoi(v[2])
		if err != nil {
			return -1, errors.Wrap(err, "convert AppArmor patch version")
		}
	}

	// major*10^5 + minor*10^3 + patch*10^0
	numericVersion := majorVersion*1e5 + minorVersion*1e3 + patchLevel
	return numericVersion, nil

}

// CheckProfileAndLoadDefault checks if the specified profile is loaded and
// loads the DefaultLibpodProfile if the specified on is prefixed by
// DefaultLipodProfilePrefix.  This allows to always load and apply the latest
// default AppArmor profile.  Note that AppArmor requires root.  If it's a
// default profile, return DefaultLipodProfilePrefix, otherwise the specified
// one.
func CheckProfileAndLoadDefault(name string) (string, error) {
	if name == "unconfined" {
		return name, nil
	}

	// AppArmor is not supported in rootless mode as it requires root
	// privileges.  Return an error in case a specific profile is specified.
	if unshare.IsRootless() {
		if name != "" {
			return "", errors.Wrapf(ErrApparmorRootless, "cannot load AppArmor profile %q", name)
		} else {
			logrus.Debug("skipping loading default AppArmor profile (rootless mode)")
			return "", nil
		}
	}

	// Check if AppArmor is disabled and error out if a profile is to be set.
	if !runcaa.IsEnabled() {
		if name == "" {
			return "", nil
		} else {
			return "", errors.Errorf("profile %q specified but AppArmor is disabled on the host", name)
		}
	}

	if name == "" {
		name = Profile
	} else if !strings.HasPrefix(name, ProfilePrefix) {
		// If the specified name is not a default one, ignore it and return the
		// name.
		isLoaded, err := IsLoaded(name)
		if err != nil {
			return "", errors.Wrapf(err, "verify if profile %s is loaded", name)
		}
		if !isLoaded {
			return "", errors.Errorf("AppArmor profile %q specified but not loaded", name)
		}
		return name, nil
	}

	// To avoid expensive redundant loads on each invocation, check
	// if it's loaded before installing it.
	isLoaded, err := IsLoaded(name)
	if err != nil {
		return "", errors.Wrapf(err, "verify if profile %s is loaded", name)
	}
	if !isLoaded {
		err = InstallDefault(name)
		if err != nil {
			return "", errors.Wrapf(err, "install profile %s", name)
		}
		logrus.Infof("successfully loaded AppAmor profile %q", name)
	} else {
		logrus.Infof("AppAmor profile %q is already loaded", name)
	}

	return name, nil
}
