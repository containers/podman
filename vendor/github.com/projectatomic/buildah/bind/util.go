package bind

import (
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/projectatomic/buildah/util"
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

func dedupeStringSlice(slice []string) []string {
	done := make([]string, 0, len(slice))
	m := make(map[string]struct{})
	for _, s := range slice {
		if _, present := m[s]; !present {
			m[s] = struct{}{}
			done = append(done, s)
		}
	}
	return done
}
