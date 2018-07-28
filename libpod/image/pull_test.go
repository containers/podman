package image

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/containers/image/transports"
	"github.com/containers/image/transports/alltransports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPullRefName(t *testing.T) {
	const imageID = "@0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	const digestSuffix = "@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	for _, c := range []struct{ srcName, destName, expectedImage, expectedDstName string }{
		// == Source does not have a Docker reference (as is the case for docker-archive:, oci-archive, dir:); destination formats:
		{ // registry/name, no tag:
			"dir:/dev/this-does-not-exist", "example.com/from-directory",
			// The destName value will be interpreted as "example.com/from-directory:latest" by storageTransport.
			"example.com/from-directory", "example.com/from-directory",
		},
		{ // name, no registry, no tag:
			"dir:/dev/this-does-not-exist", "from-directory",
			// FIXME(?) Adding a registry also adds a :latest tag.  OTOH that actually matches the used destination.
			// Either way it is surprising that the localhost/ addition changes this.  (mitr hoping to remove the "image" member).
			"localhost/from-directory:latest", "localhost/from-directory:latest",
		},
		{ // registry/name:tag :
			"dir:/dev/this-does-not-exist", "example.com/from-directory:notlatest",
			"example.com/from-directory:notlatest", "example.com/from-directory:notlatest",
		},
		{ // name:tag, no registry:
			"dir:/dev/this-does-not-exist", "from-directory:notlatest",
			"localhost/from-directory:notlatest", "localhost/from-directory:notlatest",
		},
		{ // name@digest, no registry:
			"dir:/dev/this-does-not-exist", "from-directory" + digestSuffix,
			// FIXME?! Why is this dropping the digest, and adding :none?!
			"localhost/from-directory:none", "localhost/from-directory:none",
		},
		{ // registry/name@digest:
			"dir:/dev/this-does-not-exist", "example.com/from-directory" + digestSuffix,
			"example.com/from-directory" + digestSuffix, "example.com/from-directory" + digestSuffix,
		},
		{ // ns/name:tag, no registry:
			// FIXME: This is interpreted as "registry == ns"
			"dir:/dev/this-does-not-exist", "ns/from-directory:notlatest",
			"ns/from-directory:notlatest", "ns/from-directory:notlatest",
		},
		{ // containers-storage image ID
			"dir:/dev/this-does-not-exist", imageID,
			imageID, imageID,
		},
		// == Source does have a Docker reference.
		// In that case getPullListFromRef uses the full transport:name input as a destName,
		// which would be invalid in the returned dstName - but dstName is derived from the source, so it does not really matter _so_ much.
		// Note that unlike real-world use we use different :source and :destination to verify the data flow in more detail.
		{ // registry/name:tag
			"docker://example.com/busybox:source", "docker://example.com/busybox:destination",
			"docker://example.com/busybox:destination", "example.com/busybox:source",
		},
		{ // Implied docker.io/library and :latest
			"docker://busybox", "docker://busybox:destination",
			"docker://busybox:destination", "docker.io/library/busybox:latest",
		},
	} {
		srcRef, err := alltransports.ParseImageName(c.srcName)
		require.NoError(t, err, c.srcName)

		res := getPullRefName(srcRef, c.destName)
		assert.Equal(t, pullRefName{image: c.expectedImage, srcRef: srcRef, dstName: c.expectedDstName}, res,
			fmt.Sprintf("%#v %#v", c.srcName, c.destName))
	}
}

func TestPullGoalNamesFromImageReference(t *testing.T) {
	type expected struct{ image, dstName string }
	for _, c := range []struct {
		srcName              string
		expected             []expected
		expectedPullAllPairs bool
	}{
		// == docker-archive:
		{"docker-archive:/dev/this-does-not-exist", nil, false}, // Input does not exist.
		{"docker-archive:/dev/null", nil, false},                // Input exists but does not contain a manifest.
		// FIXME: The implementation has extra code for len(manifest) == 0?! That will fail in getImageDigest anyway.
		{ // RepoTags is empty
			"docker-archive:testdata/docker-unnamed.tar.xz",
			[]expected{{"@ec9293436c2e66da44edb9efb8d41f6b13baf62283ebe846468bc992d76d7951", "@ec9293436c2e66da44edb9efb8d41f6b13baf62283ebe846468bc992d76d7951"}},
			false,
		},
		{ // RepoTags is a [docker.io/library/]name:latest, normalized to the short format.
			"docker-archive:testdata/docker-name-only.tar.xz",
			[]expected{{"localhost/pretty-empty:latest", "localhost/pretty-empty:latest"}},
			true,
		},
		{ // RepoTags is a registry/name:latest
			"docker-archive:testdata/docker-registry-name.tar.xz",
			[]expected{{"example.com/empty:latest", "example.com/empty:latest"}},
			true,
		},
		{ // RepoTags has multiple items for a single image
			"docker-archive:testdata/docker-two-names.tar.xz",
			[]expected{
				{"localhost/pretty-empty:latest", "localhost/pretty-empty:latest"},
				{"example.com/empty:latest", "example.com/empty:latest"},
			},
			true,
		},
		{ // FIXME: Two images in a single archive - only the "first" one (whichever it is) is returned
			// (and docker-archive: then refuses to read anything when the manifest has more than 1 item)
			"docker-archive:testdata/docker-two-images.tar.xz",
			[]expected{{"example.com/empty:latest", "example.com/empty:latest"}},
			// "example.com/empty/but:different" exists but is ignored
			true,
		},

		// == oci-archive:
		{"oci-archive:/dev/this-does-not-exist", nil, false}, // Input does not exist.
		{"oci-archive:/dev/null", nil, false},                // Input exists but does not contain a manifest.
		// FIXME: The remaining tests are commented out for now, because oci-archive: does not work unprivileged.
		// { // No name annotation
		// 	"oci-archive:testdata/oci-unnamed.tar.gz",
		// 	[]expected{{"@5c8aca8137ac47e84c69ae93ce650ce967917cc001ba7aad5494073fac75b8b6", "@5c8aca8137ac47e84c69ae93ce650ce967917cc001ba7aad5494073fac75b8b6"}},
		//  false,
		// },
		// { // Name is a name:latest (no normalization is defined).
		// 	"oci-archive:testdata/oci-name-only.tar.gz",
		// 	[]expected{{"localhost/pretty-empty:latest", "localhost/pretty-empty:latest"}},
		//  false,
		// },
		// { // Name is a registry/name:latest
		// 	"oci-archive:testdata/oci-registry-name.tar.gz",
		// 	[]expected{{"example.com/empty:latest", "example.com/empty:latest"}},
		//  false,
		// },
		// { // Name exists, but is an invalid Docker reference; such names are passed through, and will fail when intepreting dstName.
		// 	"oci-archive:testdata/oci-non-docker-name.tar.gz",
		// 	[]expected{{"UPPERCASE-IS-INVALID", "UPPERCASE-IS-INVALID"}},
		//  false,
		// },
		// Maybe test support of two images in a single archive? It should be transparently handled by adding a reference to srcRef.

		// == dir:
		{ // Absolute path
			"dir:/dev/this-does-not-exist",
			[]expected{{"localhost/dev/this-does-not-exist", "localhost/dev/this-does-not-exist"}},
			false,
		},
		{ // Relative path, single element.
			// FIXME? Note the :latest difference in .image.  (In .dstName as well, but it has the same semantics in there.)
			"dir:this-does-not-exist",
			[]expected{{"localhost/this-does-not-exist:latest", "localhost/this-does-not-exist:latest"}},
			false,
		},
		{ // Relative path, multiple elements.
			// FIXME: This does not add localhost/, and dstName is parsed as docker.io/testdata.
			"dir:testdata/this-does-not-exist",
			[]expected{{"testdata/this-does-not-exist", "testdata/this-does-not-exist"}},
			false,
		},

		// == Others, notably:
		// === docker:// (has ImageReference.DockerReference)
		{ // Fully-specified input
			"docker://docker.io/library/busybox:latest",
			[]expected{{"docker://docker.io/library/busybox:latest", "docker.io/library/busybox:latest"}},
			false,
		},
		{ // Minimal form of the input
			"docker://busybox",
			[]expected{{"docker://busybox", "docker.io/library/busybox:latest"}},
			false,
		},

		// === tarball: (as an example of what happens when ImageReference.DockerReference is nil).
		{ // FIXME? The dstName value is invalid, and will fail.
			// (This is NOT an API promise that the results will continue to be this way.)
			"tarball:/dev/null",
			[]expected{{"tarball:/dev/null", "tarball:/dev/null"}},
			false,
		},
	} {
		srcRef, err := alltransports.ParseImageName(c.srcName)
		require.NoError(t, err, c.srcName)

		res, err := pullGoalNamesFromImageReference(context.Background(), srcRef, c.srcName, nil)
		if len(c.expected) == 0 {
			assert.Error(t, err, c.srcName)
		} else {
			require.NoError(t, err, c.srcName)
			require.Len(t, res.refNames, len(c.expected), c.srcName)
			for i, e := range c.expected {
				assert.Equal(t, pullRefName{image: e.image, srcRef: srcRef, dstName: e.dstName}, res.refNames[i], fmt.Sprintf("%s #%d", c.srcName, i))
			}
			assert.Equal(t, c.expectedPullAllPairs, res.pullAllPairs, c.srcName)
			assert.False(t, res.usedSearchRegistries, c.srcName)
			assert.Nil(t, res.searchedRegistries, c.srcName)
		}
	}
}

const registriesConfWithSearch = `[registries.search]
registries = ['example.com', 'docker.io']
`

func TestPullGoalNamesFromPossiblyUnqualifiedName(t *testing.T) {
	const digestSuffix = "@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	type pullRefStrings struct{ image, srcRef, dstName string } // pullRefName with string data only

	registriesConf, err := ioutil.TempFile("", "TestPullGoalNamesFromPossiblyUnqualifiedName")
	require.NoError(t, err)
	defer registriesConf.Close()
	defer os.Remove(registriesConf.Name())

	err = ioutil.WriteFile(registriesConf.Name(), []byte(registriesConfWithSearch), 0600)
	require.NoError(t, err)

	// Environment is per-process, so this looks very unsafe; actually it seems fine because tests are not
	// run in parallel unless they opt in by calling t.Parallel().  So don’t do that.
	oldRCP, hasRCP := os.LookupEnv("REGISTRIES_CONFIG_PATH")
	defer func() {
		if hasRCP {
			os.Setenv("REGISTRIES_CONFIG_PATH", oldRCP)
		} else {
			os.Unsetenv("REGISTRIES_CONFIG_PATH")
		}
	}()
	os.Setenv("REGISTRIES_CONFIG_PATH", registriesConf.Name())

	for _, c := range []struct {
		input                        string
		expected                     []pullRefStrings
		expectedUsedSearchRegistries bool
	}{
		{"#", nil, false}, // Clearly invalid.
		{ // Fully-explicit docker.io, name-only.
			"docker.io/library/busybox",
			// (The docker:// representation is shortened by c/image/docker.Reference but it refers to "docker.io/library".)
			[]pullRefStrings{{"docker.io/library/busybox", "docker://busybox:latest", "docker.io/library/busybox"}},
			false,
		},
		{ // docker.io with implied /library/, name-only.
			"docker.io/busybox",
			// (The docker:// representation is shortened by c/image/docker.Reference but it refers to "docker.io/library".)
			// The .dstName fields differ for the explicit/implicit /library/ cases, but StorageTransport.ParseStoreReference normalizes that.
			[]pullRefStrings{{"docker.io/busybox", "docker://busybox:latest", "docker.io/busybox"}},
			false,
		},
		{ // Qualified example.com, name-only.
			"example.com/ns/busybox",
			[]pullRefStrings{{"example.com/ns/busybox", "docker://example.com/ns/busybox:latest", "example.com/ns/busybox"}},
			false,
		},
		{ // Qualified example.com, name:tag.
			"example.com/ns/busybox:notlatest",
			[]pullRefStrings{{"example.com/ns/busybox:notlatest", "docker://example.com/ns/busybox:notlatest", "example.com/ns/busybox:notlatest"}},
			false,
		},
		{ // Qualified example.com, name@digest.
			"example.com/ns/busybox" + digestSuffix,
			[]pullRefStrings{{"example.com/ns/busybox" + digestSuffix, "docker://example.com/ns/busybox" + digestSuffix,
				// FIXME?! Why is .dstName dropping the digest, and adding :none?!
				"example.com/ns/busybox:none"}},
			false,
		},
		// Qualified example.com, name:tag@digest.  This code is happy to try, but .srcRef parsing currently rejects such input.
		{"example.com/ns/busybox:notlatest" + digestSuffix, nil, false},
		{ // Unqualified, single-name, name-only
			"busybox",
			[]pullRefStrings{
				{"example.com/busybox:latest", "docker://example.com/busybox:latest", "example.com/busybox:latest"},
				// (The docker:// representation is shortened by c/image/docker.Reference but it refers to "docker.io/library".)
				{"docker.io/busybox:latest", "docker://busybox:latest", "docker.io/busybox:latest"},
			},
			true,
		},
		{ // Unqualified, namespaced, name-only
			"ns/busybox",
			// FIXME: This is interpreted as "registry == ns", and actual pull happens from docker.io/ns/busybox:latest;
			// example.com should be first in the list but isn't used at all.
			[]pullRefStrings{
				{"ns/busybox", "docker://ns/busybox:latest", "ns/busybox"},
			},
			false,
		},
		{ // Unqualified, name:tag
			"busybox:notlatest",
			[]pullRefStrings{
				{"example.com/busybox:notlatest", "docker://example.com/busybox:notlatest", "example.com/busybox:notlatest"},
				// (The docker:// representation is shortened by c/image/docker.Reference but it refers to "docker.io/library".)
				{"docker.io/busybox:notlatest", "docker://busybox:notlatest", "docker.io/busybox:notlatest"},
			},
			true,
		},
		{ // Unqualified, name@digest
			"busybox" + digestSuffix,
			[]pullRefStrings{
				// FIXME?! Why is .input and .dstName dropping the digest, and adding :none?!
				{"example.com/busybox:none", "docker://example.com/busybox" + digestSuffix, "example.com/busybox:none"},
				// (The docker:// representation is shortened by c/image/docker.Reference but it refers to "docker.io/library".)
				{"docker.io/busybox:none", "docker://busybox" + digestSuffix, "docker.io/busybox:none"},
			},
			true,
		},
		// Unqualified, name:tag@digest. This code is happy to try, but .srcRef parsing currently rejects such input.
		{"busybox:notlatest" + digestSuffix, nil, false},
	} {
		res, err := pullGoalNamesFromPossiblyUnqualifiedName(c.input)
		if len(c.expected) == 0 {
			assert.Error(t, err, c.input)
		} else {
			assert.NoError(t, err, c.input)
			strings := make([]pullRefStrings, len(res.refNames))
			for i, rn := range res.refNames {
				strings[i] = pullRefStrings{
					image:   rn.image,
					srcRef:  transports.ImageName(rn.srcRef),
					dstName: rn.dstName,
				}
			}
			assert.Equal(t, c.expected, strings, c.input)
			assert.False(t, res.pullAllPairs, c.input)
			assert.Equal(t, c.expectedUsedSearchRegistries, res.usedSearchRegistries, c.input)
			if !c.expectedUsedSearchRegistries {
				assert.Nil(t, res.searchedRegistries, c.input)
			} else {
				assert.Equal(t, []string{"example.com", "docker.io"}, res.searchedRegistries, c.input)
			}
		}
	}
}
