package image

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/containers/image/transports"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/idtools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestRuntime returns a *Runtime implementation and a cleanup function which the caller is expected to call.
func newTestRuntime(t *testing.T) (*Runtime, func()) {
	wd, err := ioutil.TempDir("", "testStorageRuntime")
	require.NoError(t, err)
	err = os.MkdirAll(wd, 0700)
	require.NoError(t, err)

	store, err := storage.GetStore(storage.StoreOptions{
		RunRoot:            filepath.Join(wd, "run"),
		GraphRoot:          filepath.Join(wd, "root"),
		GraphDriverName:    "vfs",
		GraphDriverOptions: []string{},
		UIDMap: []idtools.IDMap{{
			ContainerID: 0,
			HostID:      os.Getuid(),
			Size:        1,
		}},
		GIDMap: []idtools.IDMap{{
			ContainerID: 0,
			HostID:      os.Getgid(),
			Size:        1,
		}},
	})
	require.NoError(t, err)

	ir := NewImageRuntimeFromStore(store)
	cleanup := func() { _ = os.RemoveAll(wd) }
	return ir, cleanup
}

// storageReferenceWithoutLocation returns ref.StringWithinTransport(),
// stripping the [store-specification] prefix from containers/image/storage reference format.
func storageReferenceWithoutLocation(ref types.ImageReference) string {
	res := ref.StringWithinTransport()
	if res[0] == '[' {
		closeIndex := strings.IndexRune(res, ']')
		if closeIndex > 0 {
			res = res[closeIndex+1:]
		}
	}
	return res
}

func TestGetPullRefPair(t *testing.T) {
	const imageID = "@0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	const digestSuffix = "@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	ir, cleanup := newTestRuntime(t)
	defer cleanup()

	for _, c := range []struct{ srcName, destName, expectedImage, expectedDstName string }{
		// == Source does not have a Docker reference (as is the case for docker-archive:, oci-archive, dir:); destination formats:
		{ // registry/name, no tag:
			"dir:/dev/this-does-not-exist", "example.com/from-directory",
			"example.com/from-directory", "example.com/from-directory:latest",
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
			"dir:/dev/this-does-not-exist", "ns/from-directory:notlatest",
			"localhost/ns/from-directory:notlatest", "localhost/ns/from-directory:notlatest",
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
		// == Invalid destination format.
		{"tarball:/dev/null", "tarball:/dev/null", "", ""},
	} {
		testDescription := fmt.Sprintf("%#v %#v", c.srcName, c.destName)
		srcRef, err := alltransports.ParseImageName(c.srcName)
		require.NoError(t, err, testDescription)

		res, err := ir.getPullRefPair(srcRef, c.destName)
		if c.expectedDstName == "" {
			assert.Error(t, err, testDescription)
		} else {
			require.NoError(t, err, testDescription)
			assert.Equal(t, c.expectedImage, res.image, testDescription)
			assert.Equal(t, srcRef, res.srcRef, testDescription)
			assert.Equal(t, c.expectedDstName, storageReferenceWithoutLocation(res.dstRef), testDescription)
		}
	}
}

func TestPullGoalFromImageReference(t *testing.T) {
	ir, cleanup := newTestRuntime(t)
	defer cleanup()

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
		// // Name exists, but is an invalid Docker reference; such names will fail when creating dstReference.
		// {"oci-archive:testdata/oci-non-docker-name.tar.gz", nil, false},
		// Maybe test support of two images in a single archive? It should be transparently handled by adding a reference to srcRef.

		// == dir:
		{ // Absolute path
			"dir:/dev/this-does-not-exist",
			[]expected{{"localhost/dev/this-does-not-exist", "localhost/dev/this-does-not-exist:latest"}},
			false,
		},
		{ // Relative path, single element.
			// FIXME? Note the :latest difference in .image.
			"dir:this-does-not-exist",
			[]expected{{"localhost/this-does-not-exist:latest", "localhost/this-does-not-exist:latest"}},
			false,
		},
		{ // Relative path, multiple elements.
			"dir:testdata/this-does-not-exist",
			[]expected{{"localhost/testdata/this-does-not-exist:latest", "localhost/testdata/this-does-not-exist:latest"}},
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
		// FIXME? This tries to parse "tarball:/dev/null" as a storageReference, and fails.
		// (This is NOT an API promise that the results will continue to be this way.)
		{"tarball:/dev/null", nil, false},
	} {
		srcRef, err := alltransports.ParseImageName(c.srcName)
		require.NoError(t, err, c.srcName)

		res, err := ir.pullGoalFromImageReference(context.Background(), srcRef, c.srcName, nil)
		if len(c.expected) == 0 {
			assert.Error(t, err, c.srcName)
		} else {
			require.NoError(t, err, c.srcName)
			require.Len(t, res.refPairs, len(c.expected), c.srcName)
			for i, e := range c.expected {
				testDescription := fmt.Sprintf("%s #%d", c.srcName, i)
				assert.Equal(t, e.image, res.refPairs[i].image, testDescription)
				assert.Equal(t, srcRef, res.refPairs[i].srcRef, testDescription)
				assert.Equal(t, e.dstName, storageReferenceWithoutLocation(res.refPairs[i].dstRef), testDescription)
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

func TestPullGoalFromPossiblyUnqualifiedName(t *testing.T) {
	const digestSuffix = "@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	type pullRefStrings struct{ image, srcRef, dstName string } // pullRefPair with string data only

	registriesConf, err := ioutil.TempFile("", "TestPullGoalFromPossiblyUnqualifiedName")
	require.NoError(t, err)
	defer registriesConf.Close()
	defer os.Remove(registriesConf.Name())

	err = ioutil.WriteFile(registriesConf.Name(), []byte(registriesConfWithSearch), 0600)
	require.NoError(t, err)

	ir, cleanup := newTestRuntime(t)
	defer cleanup()

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
			[]pullRefStrings{{"docker.io/library/busybox", "docker://busybox:latest", "docker.io/library/busybox:latest"}},
			false,
		},
		{ // docker.io with implied /library/, name-only.
			"docker.io/busybox",
			// (The docker:// representation is shortened by c/image/docker.Reference but it refers to "docker.io/library".)
			[]pullRefStrings{{"docker.io/busybox", "docker://busybox:latest", "docker.io/library/busybox:latest"}},
			false,
		},
		{ // Qualified example.com, name-only.
			"example.com/ns/busybox",
			[]pullRefStrings{{"example.com/ns/busybox", "docker://example.com/ns/busybox:latest", "example.com/ns/busybox:latest"}},
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
				{"docker.io/busybox:latest", "docker://busybox:latest", "docker.io/library/busybox:latest"},
			},
			true,
		},
		{ // Unqualified, namespaced, name-only
			"ns/busybox",
			[]pullRefStrings{
				{"example.com/ns/busybox:latest", "docker://example.com/ns/busybox:latest", "example.com/ns/busybox:latest"},
			},
			true,
		},
		{ // Unqualified, name:tag
			"busybox:notlatest",
			[]pullRefStrings{
				{"example.com/busybox:notlatest", "docker://example.com/busybox:notlatest", "example.com/busybox:notlatest"},
				// (The docker:// representation is shortened by c/image/docker.Reference but it refers to "docker.io/library".)
				{"docker.io/busybox:notlatest", "docker://busybox:notlatest", "docker.io/library/busybox:notlatest"},
			},
			true,
		},
		{ // Unqualified, name@digest
			"busybox" + digestSuffix,
			[]pullRefStrings{
				// FIXME?! Why is .input and .dstName dropping the digest, and adding :none?!
				{"example.com/busybox:none", "docker://example.com/busybox" + digestSuffix, "example.com/busybox:none"},
				// (The docker:// representation is shortened by c/image/docker.Reference but it refers to "docker.io/library".)
				{"docker.io/busybox:none", "docker://busybox" + digestSuffix, "docker.io/library/busybox:none"},
			},
			true,
		},
		// Unqualified, name:tag@digest. This code is happy to try, but .srcRef parsing currently rejects such input.
		{"busybox:notlatest" + digestSuffix, nil, false},
	} {
		res, err := ir.pullGoalFromPossiblyUnqualifiedName(c.input)
		if len(c.expected) == 0 {
			assert.Error(t, err, c.input)
		} else {
			assert.NoError(t, err, c.input)
			for i, e := range c.expected {
				testDescription := fmt.Sprintf("%s #%d", c.input, i)
				assert.Equal(t, e.image, res.refPairs[i].image, testDescription)
				assert.Equal(t, e.srcRef, transports.ImageName(res.refPairs[i].srcRef), testDescription)
				assert.Equal(t, e.dstName, storageReferenceWithoutLocation(res.refPairs[i].dstRef), testDescription)
			}
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
