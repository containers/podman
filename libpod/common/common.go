package common

import (
	"io"
	"strings"
	"syscall"

	cp "github.com/containers/image/copy"
	"github.com/containers/image/signature"
	"github.com/containers/image/types"
	"github.com/pkg/errors"
)

var (
	// ErrNoPassword is returned if the user did not supply a password
	ErrNoPassword = errors.Wrapf(syscall.EINVAL, "password was not supplied")
)

// GetCopyOptions constructs a new containers/image/copy.Options{} struct from the given parameters
func GetCopyOptions(reportWriter io.Writer, signaturePolicyPath string, srcDockerRegistry, destDockerRegistry *DockerRegistryOptions, signing SigningOptions, authFile string) *cp.Options {
	if srcDockerRegistry == nil {
		srcDockerRegistry = &DockerRegistryOptions{}
	}
	if destDockerRegistry == nil {
		destDockerRegistry = &DockerRegistryOptions{}
	}
	srcContext := srcDockerRegistry.GetSystemContext(signaturePolicyPath, authFile)
	destContext := destDockerRegistry.GetSystemContext(signaturePolicyPath, authFile)
	return &cp.Options{
		RemoveSignatures: signing.RemoveSignatures,
		SignBy:           signing.SignBy,
		ReportWriter:     reportWriter,
		SourceCtx:        srcContext,
		DestinationCtx:   destContext,
	}
}

// GetSystemContext Constructs a new containers/image/types.SystemContext{} struct from the given signaturePolicy path
func GetSystemContext(signaturePolicyPath, authFilePath string) *types.SystemContext {
	sc := &types.SystemContext{}
	if signaturePolicyPath != "" {
		sc.SignaturePolicyPath = signaturePolicyPath
	}
	sc.AuthFilePath = authFilePath
	return sc
}

// CopyStringStringMap deep copies a map[string]string and returns the result
func CopyStringStringMap(m map[string]string) map[string]string {
	n := map[string]string{}
	for k, v := range m {
		n[k] = v
	}
	return n
}

// IsTrue determines whether the given string equals "true"
func IsTrue(str string) bool {
	return str == "true"
}

// IsFalse determines whether the given string equals "false"
func IsFalse(str string) bool {
	return str == "false"
}

// IsValidBool determines whether the given string equals "true" or "false"
func IsValidBool(str string) bool {
	return IsTrue(str) || IsFalse(str)
}

// GetPolicyContext creates a signature policy context for the given signature policy path
func GetPolicyContext(path string) (*signature.PolicyContext, error) {
	policy, err := signature.DefaultPolicy(&types.SystemContext{SignaturePolicyPath: path})
	if err != nil {
		return nil, err
	}
	return signature.NewPolicyContext(policy)
}

// ParseRegistryCreds takes a credentials string in the form USERNAME:PASSWORD
// and returns a DockerAuthConfig
func ParseRegistryCreds(creds string) (*types.DockerAuthConfig, error) {
	if creds == "" {
		return nil, errors.New("no credentials supplied")
	}
	if !strings.Contains(creds, ":") {
		return &types.DockerAuthConfig{
			Username: creds,
			Password: "",
		}, ErrNoPassword
	}
	v := strings.SplitN(creds, ":", 2)
	cfg := &types.DockerAuthConfig{
		Username: v[0],
		Password: v[1],
	}
	return cfg, nil
}
