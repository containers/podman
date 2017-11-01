// +build apparmor

package apparmor

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/docker/docker/utils/templates"
	"github.com/opencontainers/runc/libcontainer/apparmor"
)

const (
	// profileDirectory is the file store for apparmor profiles and macros.
	profileDirectory = "/etc/apparmor.d"
)

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

// EnsureDefaultApparmorProfile loads default apparmor profile, if it is not loaded.
func EnsureDefaultApparmorProfile() error {
	if apparmor.IsEnabled() {
		loaded, err := IsLoaded(DefaultApparmorProfile)
		if err != nil {
			return fmt.Errorf("Could not check if %s AppArmor profile was loaded: %s", DefaultApparmorProfile, err)
		}

		// Nothing to do.
		if loaded {
			return nil
		}

		// Load the profile.
		if err := InstallDefault(DefaultApparmorProfile); err != nil {
			return fmt.Errorf("AppArmor enabled on system but the %s profile could not be loaded.", DefaultApparmorProfile)
		}
	}

	return nil
}

// IsEnabled returns true if apparmor is enabled for the host.
func IsEnabled() bool {
	return apparmor.IsEnabled()
}

// GetProfileNameFromPodAnnotations gets the name of the profile to use with container from
// pod annotations
func GetProfileNameFromPodAnnotations(annotations map[string]string, containerName string) string {
	return annotations[ContainerAnnotationKeyPrefix+containerName]
}

// InstallDefault generates a default profile in a temp directory determined by
// os.TempDir(), then loads the profile into the kernel using 'apparmor_parser'.
func InstallDefault(name string) error {
	p := profileData{
		Name: name,
	}

	// Install to a temporary directory.
	f, err := ioutil.TempFile("", name)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := p.generateDefault(f); err != nil {
		return err
	}

	return LoadProfile(f.Name())
}

// IsLoaded checks if a profile with the given name has been loaded into the
// kernel.
func IsLoaded(name string) (bool, error) {
	file, err := os.Open("/sys/kernel/security/apparmor/profiles")
	if err != nil {
		return false, err
	}
	defer file.Close()

	r := bufio.NewReader(file)
	for {
		p, err := r.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, err
		}
		if strings.HasPrefix(p, name+" ") {
			return true, nil
		}
	}

	return false, nil
}

// generateDefault creates an apparmor profile from ProfileData.
func (p *profileData) generateDefault(out io.Writer) error {
	compiled, err := templates.NewParse("apparmor_profile", baseTemplate)
	if err != nil {
		return err
	}

	if macroExists("tunables/global") {
		p.Imports = append(p.Imports, "#include <tunables/global>")
	} else {
		p.Imports = append(p.Imports, "@{PROC}=/proc/")
	}

	if macroExists("abstractions/base") {
		p.InnerImports = append(p.InnerImports, "#include <abstractions/base>")
	}

	ver, err := GetVersion()
	if err != nil {
		return err
	}
	p.Version = ver

	return compiled.Execute(out, p)
}

// macrosExists checks if the passed macro exists.
func macroExists(m string) bool {
	_, err := os.Stat(path.Join(profileDirectory, m))
	return err == nil
}
