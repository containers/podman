// +build !seccomp

package seccomp // import "github.com/seccomp/containers-golang"

import (
	"fmt"

	"github.com/opencontainers/runtime-spec/specs-go"
)

// DefaultProfile returns a nil pointer on unsupported systems.
func DefaultProfile() *Seccomp {
	return nil
}

// LoadProfile returns an error on unsuppored systems
func LoadProfile(body string, rs *specs.Spec) (*specs.LinuxSeccomp, error) {
	return nil, fmt.Errorf("Seccomp not supported on this platform")
}

// GetDefaultProfile returns an error on unsuppored systems
func GetDefaultProfile(rs *specs.Spec) (*specs.LinuxSeccomp, error) {
	return nil, fmt.Errorf("Seccomp not supported on this platform")
}
