//go:build !remote

package util

import (
	"strings"

	"github.com/opencontainers/runtime-spec/specs-go"
)

// FormatRlimits formats the rlimits to the runtime spec format
func FormatRlimits(rlimits []specs.POSIXRlimit) []specs.POSIXRlimit {
	formatted := make([]specs.POSIXRlimit, len(rlimits))
	for i, rlimit := range rlimits {
		typ := strings.ToUpper(rlimit.Type)
		if !strings.HasPrefix(typ, "RLIMIT_") {
			typ = "RLIMIT_" + typ
		}
		formatted[i] = specs.POSIXRlimit{
			Type: typ,
			Hard: rlimit.Hard,
			Soft: rlimit.Soft,
		}
	}
	return formatted
}
