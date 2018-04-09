package buildah

import (
	"github.com/containers/image/docker/reference"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/reexec"
	"github.com/pkg/errors"
)

// InitReexec is a wrapper for reexec.Init().  It should be called at
// the start of main(), and if it returns true, main() should return
// immediately.
func InitReexec() bool {
	return reexec.Init()
}

func copyStringStringMap(m map[string]string) map[string]string {
	n := map[string]string{}
	for k, v := range m {
		n[k] = v
	}
	return n
}

func copyStringSlice(s []string) []string {
	t := make([]string, len(s))
	copy(t, s)
	return t
}

// AddImageNames adds the specified names to the specified image.
func AddImageNames(store storage.Store, image *storage.Image, addNames []string) error {
	names, err := ExpandNames(addNames)
	if err != nil {
		return err
	}
	err = store.SetNames(image.ID, append(image.Names, names...))
	if err != nil {
		return errors.Wrapf(err, "error adding names (%v) to image %q", names, image.ID)
	}
	return nil
}

// ExpandNames takes unqualified names, parses them as image names, and returns
// the fully expanded result, including a tag.  Names which don't include a registry
// name will be marked for the most-preferred registry (i.e., the first one in our
// configuration).
func ExpandNames(names []string) ([]string, error) {
	expanded := make([]string, 0, len(names))
	for _, n := range names {
		name, err := reference.ParseNormalizedNamed(n)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing name %q", n)
		}
		name = reference.TagNameOnly(name)
		tag := ""
		digest := ""
		if tagged, ok := name.(reference.NamedTagged); ok {
			tag = ":" + tagged.Tag()
		}
		if digested, ok := name.(reference.Digested); ok {
			digest = "@" + digested.Digest().String()
		}
		expanded = append(expanded, name.Name()+tag+digest)
	}
	return expanded, nil
}
