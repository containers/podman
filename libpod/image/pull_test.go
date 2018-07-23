package image

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/containers/image/transports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
