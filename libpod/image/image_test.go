package image

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/containers/podman/v3/libpod/events"
	"github.com/containers/podman/v3/pkg/util"
	podmanVersion "github.com/containers/podman/v3/version"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/reexec"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	bbNames      = []string{"docker.io/library/busybox:latest", "docker.io/library/busybox", "docker.io/busybox:latest", "docker.io/busybox", "busybox:latest", "busybox"}
	bbGlibcNames = []string{"docker.io/library/busybox:glibc", "docker.io/busybox:glibc", "busybox:glibc"}
)

type localImageTest struct {
	fqname, taggedName string
	img                *Image
	names              []string
}

// make a temporary directory for the runtime
func mkWorkDir() (string, error) {
	return ioutil.TempDir("", "podman-test")
}

// shutdown the runtime and clean behind it
func cleanup(workdir string, ir *Runtime) {
	if err := ir.Shutdown(false); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err := os.RemoveAll(workdir)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func makeLocalMatrix(b, bg *Image) []localImageTest {
	var l []localImageTest
	// busybox
	busybox := localImageTest{
		fqname:     "docker.io/library/busybox:latest",
		taggedName: "bb:latest",
	}
	busybox.img = b
	busybox.names = b.Names()
	busybox.names = append(busybox.names, []string{"bb:latest", "bb", b.ID(), b.ID()[0:7], fmt.Sprintf("busybox@%s", b.Digest())}...)

	// busybox-glibc
	busyboxGlibc := localImageTest{
		fqname:     "docker.io/library/busybox:glibc",
		taggedName: "bb:glibc",
	}

	busyboxGlibc.img = bg
	busyboxGlibc.names = bbGlibcNames

	l = append(l, busybox, busyboxGlibc)
	return l
}

func TestMain(m *testing.M) {
	if reexec.Init() {
		return
	}
	os.Exit(m.Run())
}

// TestImage_NewFromLocal tests finding the image locally by various names,
// tags, and aliases
func TestImage_NewFromLocal(t *testing.T) {
	if os.Geteuid() != 0 { // containers/storage requires root access
		t.Skipf("Test not running as root")
	}

	workdir, err := mkWorkDir()
	assert.NoError(t, err)
	so := storage.StoreOptions{
		RunRoot:   workdir,
		GraphRoot: workdir,
	}
	writer := os.Stdout

	// Need images to be present for this test
	ir, err := NewImageRuntimeFromOptions(so)
	assert.NoError(t, err)
	defer cleanup(workdir, ir)

	ir.Eventer = events.NewNullEventer()
	bb, err := ir.New(context.Background(), "docker.io/library/busybox:latest", "", "", writer, nil, SigningOptions{}, nil, util.PullImageMissing, nil)
	assert.NoError(t, err)
	bbglibc, err := ir.New(context.Background(), "docker.io/library/busybox:glibc", "", "", writer, nil, SigningOptions{}, nil, util.PullImageMissing, nil)
	assert.NoError(t, err)

	tm := makeLocalMatrix(bb, bbglibc)
	for _, image := range tm {
		// tag our images
		err = image.img.TagImage(image.taggedName)
		assert.NoError(t, err)
		for _, name := range image.names {
			newImage, err := ir.NewFromLocal(name)
			require.NoError(t, err)
			assert.Equal(t, newImage.ID(), image.img.ID())
		}
	}
}

// TestImage_New tests pulling the image by various names, tags, and from
// different registries
func TestImage_New(t *testing.T) {
	if os.Geteuid() != 0 { // containers/storage requires root access
		t.Skipf("Test not running as root")
	}

	var names []string
	workdir, err := mkWorkDir()
	assert.NoError(t, err)
	so := storage.StoreOptions{
		RunRoot:   workdir,
		GraphRoot: workdir,
	}
	ir, err := NewImageRuntimeFromOptions(so)
	assert.NoError(t, err)
	defer cleanup(workdir, ir)

	ir.Eventer = events.NewNullEventer()
	// Build the list of pull names
	names = append(names, bbNames...)
	writer := os.Stdout

	opts := DockerRegistryOptions{
		RegistriesConfPath: "testdata/registries.conf",
	}
	// Iterate over the names and delete the image
	// after the pull
	for _, img := range names {
		newImage, err := ir.New(context.Background(), img, "", "", writer, &opts, SigningOptions{}, nil, util.PullImageMissing, nil)
		require.NoError(t, err, img)
		assert.NotEqual(t, newImage.ID(), "")
		err = newImage.Remove(context.Background(), false)
		assert.NoError(t, err)
	}
}

// TestImage_MatchRepoTag tests the various inputs we need to match
// against an image's reponames
func TestImage_MatchRepoTag(t *testing.T) {
	if os.Geteuid() != 0 { // containers/storage requires root access
		t.Skipf("Test not running as root")
	}

	//Set up
	workdir, err := mkWorkDir()
	assert.NoError(t, err)
	so := storage.StoreOptions{
		RunRoot:   workdir,
		GraphRoot: workdir,
	}
	ir, err := NewImageRuntimeFromOptions(so)
	require.NoError(t, err)
	defer cleanup(workdir, ir)

	opts := DockerRegistryOptions{
		RegistriesConfPath: "testdata/registries.conf",
	}
	ir.Eventer = events.NewNullEventer()
	newImage, err := ir.New(context.Background(), "busybox", "", "", os.Stdout, &opts, SigningOptions{}, nil, util.PullImageMissing, nil)
	require.NoError(t, err)
	err = newImage.TagImage("foo:latest")
	require.NoError(t, err)
	err = newImage.TagImage("foo:bar")
	require.NoError(t, err)

	// Tests start here.
	for _, name := range bbNames {
		repoTag, err := newImage.MatchRepoTag(name)
		assert.NoError(t, err)
		assert.Equal(t, "docker.io/library/busybox:latest", repoTag)
	}

	// Test against tagged images of busybox

	// foo should resolve to foo:latest
	repoTag, err := newImage.MatchRepoTag("foo")
	require.NoError(t, err)
	assert.Equal(t, "localhost/foo:latest", repoTag)

	// foo:bar should resolve to foo:bar
	repoTag, err = newImage.MatchRepoTag("foo:bar")
	require.NoError(t, err)
	assert.Equal(t, "localhost/foo:bar", repoTag)
}

// TestImage_RepoDigests tests RepoDigest generation.
func TestImage_RepoDigests(t *testing.T) {
	dgst, err := digest.Parse("sha256:7173b809ca12ec5dee4506cd86be934c4596dd234ee82c0662eac04a8c2c71dc")
	require.NoError(t, err)

	for _, tt := range []struct {
		name     string
		names    []string
		expected []string
	}{
		{
			name:     "empty",
			names:    []string{},
			expected: nil,
		},
		{
			name:     "tagged",
			names:    []string{"docker.io/library/busybox:latest"},
			expected: []string{"docker.io/library/busybox@sha256:7173b809ca12ec5dee4506cd86be934c4596dd234ee82c0662eac04a8c2c71dc"},
		},
		{
			name:     "digest",
			names:    []string{"docker.io/library/busybox@sha256:7173b809ca12ec5dee4506cd86be934c4596dd234ee82c0662eac04a8c2c71dc"},
			expected: []string{"docker.io/library/busybox@sha256:7173b809ca12ec5dee4506cd86be934c4596dd234ee82c0662eac04a8c2c71dc"},
		},
	} {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			image := &Image{
				image: &storage.Image{
					Names:  test.names,
					Digest: dgst,
				},
			}
			actual, err := image.RepoDigests()
			require.NoError(t, err)
			assert.Equal(t, test.expected, actual)

			image = &Image{
				image: &storage.Image{
					Names:   test.names,
					Digests: []digest.Digest{dgst},
				},
			}
			actual, err = image.RepoDigests()
			require.NoError(t, err)
			assert.Equal(t, test.expected, actual)
		})
	}
}

// Test_splitString tests the splitString function in image that
// takes input and splits on / and returns the last array item
func Test_splitString(t *testing.T) {
	assert.Equal(t, splitString("foo/bar"), "bar")
	assert.Equal(t, splitString("a/foo/bar"), "bar")
	assert.Equal(t, splitString("bar"), "bar")
}

// Test_stripSha256 tests test the stripSha256 function which removes
// the prefix "sha256:" from a string if it is present
func Test_stripSha256(t *testing.T) {
	assert.Equal(t, stripSha256(""), "")
	assert.Equal(t, stripSha256("test1"), "test1")
	assert.Equal(t, stripSha256("sha256:9110ae7f579f35ee0c3938696f23fe0f5fbe641738ea52eb83c2df7e9995fa17"), "9110ae7f579f35ee0c3938696f23fe0f5fbe641738ea52eb83c2df7e9995fa17")
	assert.Equal(t, stripSha256("sha256:9110ae7f"), "9110ae7f")
	assert.Equal(t, stripSha256("sha256:"), "sha256:")
	assert.Equal(t, stripSha256("sha256:a"), "a")
}

func TestNormalizedTag(t *testing.T) {
	const digestSuffix = "@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	for _, c := range []struct{ input, expected string }{
		{"#", ""}, // Clearly invalid
		{"example.com/busybox", "example.com/busybox:latest"},                                            // Qualified name-only
		{"example.com/busybox:notlatest", "example.com/busybox:notlatest"},                               // Qualified name:tag
		{"example.com/busybox" + digestSuffix, "example.com/busybox" + digestSuffix},                     // Qualified name@digest; FIXME? Should we allow tagging with a digest at all?
		{"example.com/busybox:notlatest" + digestSuffix, "example.com/busybox:notlatest" + digestSuffix}, // Qualified name:tag@digest
		{"busybox:latest", "localhost/busybox:latest"},                                                   // Unqualified name-only
		{"ns/busybox:latest", "localhost/ns/busybox:latest"},                                             // Unqualified with a dot-less namespace
		{"docker.io/busybox:latest", "docker.io/library/busybox:latest"},                                 // docker.io without /library/
	} {
		res, err := NormalizedTag(c.input)
		if c.expected == "" {
			assert.Error(t, err, c.input)
		} else {
			assert.NoError(t, err, c.input)
			assert.Equal(t, c.expected, res.String())
		}
	}
}

func TestGetSystemContext(t *testing.T) {
	sc := GetSystemContext("", "", false)
	assert.Equal(t, sc.SignaturePolicyPath, "")
	assert.Equal(t, sc.AuthFilePath, "")
	assert.Equal(t, sc.DirForceCompress, false)
	assert.Equal(t, sc.DockerRegistryUserAgent, fmt.Sprintf("libpod/%s", podmanVersion.Version))
	assert.Equal(t, sc.BigFilesTemporaryDir, "/var/tmp")

	oldtmpdir := os.Getenv("TMPDIR")
	os.Setenv("TMPDIR", "/mnt")
	sc = GetSystemContext("/tmp/foo", "/tmp/bar", true)
	assert.Equal(t, sc.SignaturePolicyPath, "/tmp/foo")
	assert.Equal(t, sc.AuthFilePath, "/tmp/bar")
	assert.Equal(t, sc.DirForceCompress, true)
	assert.Equal(t, sc.BigFilesTemporaryDir, "/mnt")
	if oldtmpdir != "" {
		os.Setenv("TMPDIR", oldtmpdir)
	} else {
		os.Unsetenv("TMPDIR")
	}
}
