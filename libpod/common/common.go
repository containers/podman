package common

import (
	"github.com/containers/image/types"
)

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
