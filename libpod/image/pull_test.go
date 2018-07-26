package image

import (
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

		testName := fmt.Sprintf("%#v %#v", c.srcName, c.destName)
		res, err := getPullRefName(srcRef, c.destName)
		require.NoError(t, err, testName)
		assert.Equal(t, &pullRefName{image: c.expectedImage, srcRef: srcRef, dstName: c.expectedDstName}, res, testName)
	}
}

const registriesConfWithSearch = `[registries.search]
registries = ['example.com', 'docker.io']
`

func TestRefNamesFromPossiblyUnqualifiedName(t *testing.T) {
	const digestSuffix = "@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	type pullRefStrings struct{ image, srcRef, dstName string } // pullRefName with string data only

	registriesConf, err := ioutil.TempFile("", "TestRefNamesFromPossiblyUnqualifiedName")
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
		input    string
		expected []pullRefStrings
	}{
		{"#", nil}, // Clearly invalid.
		{ // Fully-explicit docker.io, name-only.
			"docker.io/library/busybox",
			// (The docker:// representation is shortened by c/image/docker.Reference but it refers to "docker.io/library".)
			[]pullRefStrings{{"docker.io/library/busybox", "docker://busybox:latest", "docker.io/library/busybox"}},
		},
		{ // docker.io with implied /library/, name-only.
			"docker.io/busybox",
			// (The docker:// representation is shortened by c/image/docker.Reference but it refers to "docker.io/library".)
			// The .dstName fields differ for the explicit/implicit /library/ cases, but StorageTransport.ParseStoreReference normalizes that.
			[]pullRefStrings{{"docker.io/busybox", "docker://busybox:latest", "docker.io/busybox"}},
		},
		{ // Qualified example.com, name-only.
			"example.com/ns/busybox",
			[]pullRefStrings{{"example.com/ns/busybox", "docker://example.com/ns/busybox:latest", "example.com/ns/busybox"}},
		},
		{ // Qualified example.com, name:tag.
			"example.com/ns/busybox:notlatest",
			[]pullRefStrings{{"example.com/ns/busybox:notlatest", "docker://example.com/ns/busybox:notlatest", "example.com/ns/busybox:notlatest"}},
		},
		{ // Qualified example.com, name@digest.
			"example.com/ns/busybox" + digestSuffix,
			[]pullRefStrings{{"example.com/ns/busybox" + digestSuffix, "docker://example.com/ns/busybox" + digestSuffix,
				// FIXME?! Why is .dstName dropping the digest, and adding :none?!
				"example.com/ns/busybox:none"}},
		},
		// Qualified example.com, name:tag@digest.  This code is happy to try, but .srcRef parsing currently rejects such input.
		{"example.com/ns/busybox:notlatest" + digestSuffix, nil},
		{ // Unqualified, single-name, name-only
			"busybox",
			[]pullRefStrings{
				{"example.com/busybox:latest", "docker://example.com/busybox:latest", "example.com/busybox:latest"},
				// (The docker:// representation is shortened by c/image/docker.Reference but it refers to "docker.io/library".)
				{"docker.io/busybox:latest", "docker://busybox:latest", "docker.io/busybox:latest"},
			},
		},
		{ // Unqualified, namespaced, name-only
			"ns/busybox",
			// FIXME: This is interpreted as "registry == ns", and actual pull happens from docker.io/ns/busybox:latest;
			// example.com should be first in the list but isn't used at all.
			[]pullRefStrings{
				{"ns/busybox", "docker://ns/busybox:latest", "ns/busybox"},
			},
		},
		{ // Unqualified, name:tag
			"busybox:notlatest",
			[]pullRefStrings{
				{"example.com/busybox:notlatest", "docker://example.com/busybox:notlatest", "example.com/busybox:notlatest"},
				// (The docker:// representation is shortened by c/image/docker.Reference but it refers to "docker.io/library".)
				{"docker.io/busybox:notlatest", "docker://busybox:notlatest", "docker.io/busybox:notlatest"},
			},
		},
		{ // Unqualified, name@digest
			"busybox" + digestSuffix,
			[]pullRefStrings{
				// FIXME?! Why is .input and .dstName dropping the digest, and adding :none?!
				{"example.com/busybox:none", "docker://example.com/busybox" + digestSuffix, "example.com/busybox:none"},
				// (The docker:// representation is shortened by c/image/docker.Reference but it refers to "docker.io/library".)
				{"docker.io/busybox:none", "docker://busybox" + digestSuffix, "docker.io/busybox:none"},
			},
		},
		// Unqualified, name:tag@digest. This code is happy to try, but .srcRef parsing currently rejects such input.
		{"busybox:notlatest" + digestSuffix, nil},
	} {
		res, err := refNamesFromPossiblyUnqualifiedName(c.input)
		if len(c.expected) == 0 {
			assert.Error(t, err, c.input)
		} else {
			assert.NoError(t, err, c.input)
			strings := make([]pullRefStrings, len(res))
			for i, rn := range res {
				strings[i] = pullRefStrings{
					image:   rn.image,
					srcRef:  transports.ImageName(rn.srcRef),
					dstName: rn.dstName,
				}
			}
			assert.Equal(t, c.expected, strings, c.input)
		}
	}
}
