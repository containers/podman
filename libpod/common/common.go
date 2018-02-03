package common

import (
	"io"

	cp "github.com/containers/image/copy"
	"github.com/containers/image/types"
)

// GetCopyOptions constructs a new containers/image/copy.Options{} struct from the given parameters
func GetCopyOptions(reportWriter io.Writer, signaturePolicyPath string, srcDockerRegistry, destDockerRegistry *DockerRegistryOptions, signing SigningOptions, authFile, manifestType string, forceCompress bool) *cp.Options {
	if srcDockerRegistry == nil {
		srcDockerRegistry = &DockerRegistryOptions{}
	}
	if destDockerRegistry == nil {
		destDockerRegistry = &DockerRegistryOptions{}
	}
	srcContext := srcDockerRegistry.GetSystemContext(signaturePolicyPath, authFile, forceCompress)
	destContext := destDockerRegistry.GetSystemContext(signaturePolicyPath, authFile, forceCompress)
	return &cp.Options{
		RemoveSignatures:      signing.RemoveSignatures,
		SignBy:                signing.SignBy,
		ReportWriter:          reportWriter,
		SourceCtx:             srcContext,
		DestinationCtx:        destContext,
		ForceManifestMIMEType: manifestType,
	}
}

// GetSystemContext Constructs a new containers/image/types.SystemContext{} struct from the given signaturePolicy path
func GetSystemContext(signaturePolicyPath, authFilePath string, forceCompress bool) *types.SystemContext {
	sc := &types.SystemContext{}
	if signaturePolicyPath != "" {
		sc.SignaturePolicyPath = signaturePolicyPath
	}
	sc.AuthFilePath = authFilePath
	sc.DirForceCompress = forceCompress
	return sc
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
