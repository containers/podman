package bind

import (
	"github.com/containers/buildah/util"
	"github.com/opencontainers/runtime-spec/specs-go"
)

const (
	// NoBindOption is an option which, if present in a Mount structure's
	// options list, will cause SetupIntermediateMountNamespace to not
	// redirect it through a bind mount.
	NoBindOption = "nobuildahbind"
)

func stripNoBindOption(spec *specs.Spec) {
	for i := range spec.Mounts {
		if util.StringInSlice(NoBindOption, spec.Mounts[i].Options) {
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
