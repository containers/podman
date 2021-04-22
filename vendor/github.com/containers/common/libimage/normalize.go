package libimage

import (
	"strings"

	"github.com/containers/image/v5/docker/reference"
	"github.com/pkg/errors"
)

// NormalizeName normalizes the provided name according to the conventions by
// Podman and Buildah.  If tag and digest are missing, the "latest" tag will be
// used.  If it's a short name, it will be prefixed with "localhost/".
//
// References to docker.io are normalized according to the Docker conventions.
// For instance, "docker.io/foo" turns into "docker.io/library/foo".
func NormalizeName(name string) (reference.Named, error) {
	// NOTE: this code is in symmetrie with containers/image/pkg/shortnames.
	ref, err := reference.Parse(name)
	if err != nil {
		return nil, errors.Wrapf(err, "error normalizing name %q", name)
	}

	named, ok := ref.(reference.Named)
	if !ok {
		return nil, errors.Errorf("%q is not a named reference", name)
	}

	// Enforce "localhost" if needed.
	registry := reference.Domain(named)
	if !(strings.ContainsAny(registry, ".:") || registry == "localhost") {
		name = toLocalImageName(ref.String())
	}

	// Another parse which also makes sure that docker.io references are
	// correctly normalized (e.g., docker.io/alpine to
	// docker.io/library/alpine).
	named, err = reference.ParseNormalizedNamed(name)
	if err != nil {
		return nil, err
	}

	if _, hasTag := named.(reference.NamedTagged); hasTag {
		return named, nil
	}
	if _, hasDigest := named.(reference.Digested); hasDigest {
		return named, nil
	}

	// Make sure to tag "latest".
	return reference.TagNameOnly(named), nil
}

// prefix the specified name with "localhost/".
func toLocalImageName(name string) string {
	return "localhost/" + strings.TrimLeft(name, "/")
}

// NameTagPair represents a RepoTag of an image.
type NameTagPair struct {
	// Name of the RepoTag. Maybe "<none>".
	Name string
	// Tag of the RepoTag. Maybe "<none>".
	Tag string

	// for internal use
	named reference.Named
}

// ToNameTagsPairs splits repoTags into name&tag pairs.
// Guaranteed to return at least one pair.
func ToNameTagPairs(repoTags []reference.Named) ([]NameTagPair, error) {
	none := "<none>"

	var pairs []NameTagPair
	for i, named := range repoTags {
		pair := NameTagPair{
			Name:  named.Name(),
			Tag:   none,
			named: repoTags[i],
		}

		if tagged, isTagged := named.(reference.NamedTagged); isTagged {
			pair.Tag = tagged.Tag()
		}
		pairs = append(pairs, pair)
	}

	if len(pairs) == 0 {
		pairs = append(pairs, NameTagPair{Name: none, Tag: none})
	}
	return pairs, nil
}
