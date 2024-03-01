package bind

import (
	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/exp/slices"
)

const (
	// NoBindOption is an option which, if present in a Mount structure's
	// options list, will cause SetupIntermediateMountNamespace to not
	// redirect it through a bind mount.
	NoBindOption = "nobuildahbind"
)

func stripNoBindOption(spec *specs.Spec) {
	for i := range spec.Mounts {
		if slices.Contains(spec.Mounts[i].Options, NoBindOption) {
			prunedOptions := make([]string, 0, len(spec.Mounts[i].Options))
			for _, option := range spec.Mounts[i].Options {
				if option != NoBindOption {
					prunedOptions = append(prunedOptions, option)
				}
			}
			spec.Mounts[i].Options = prunedOptions
		}
	}
}
