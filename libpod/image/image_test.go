package image

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/containers/storage"
	"github.com/stretchr/testify/assert"
)

var (
	bbNames      = []string{"docker.io/library/busybox:latest", "docker.io/library/busybox", "docker.io/busybox:latest", "docker.io/busybox", "busybox:latest", "busybox"}
	bbGlibcNames = []string{"docker.io/library/busybox:glibc", "docker.io/busybox:glibc", "busybox:glibc"}
	fedoraNames  = []string{"registry.fedoraproject.org/fedora-minimal:latest", "registry.fedoraproject.org/fedora-minimal", "fedora-minimal:latest", "fedora-minimal"}
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

func makeLocalMatrix(b, bg *Image) ([]localImageTest, error) {
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
	return l, nil

}

// TestImage_NewFromLocal tests finding the image locally by various names,
// tags, and aliases
func TestImage_NewFromLocal(t *testing.T) {
	workdir, err := mkWorkDir()
	assert.NoError(t, err)
	so := storage.StoreOptions{
		RunRoot:   workdir,
		GraphRoot: workdir,
	}
	var writer io.Writer
	writer = os.Stdout

	// Need images to be present for this test
	ir, err := NewImageRuntimeFromOptions(so)
	assert.NoError(t, err)
	bb, err := ir.New(context.Background(), "docker.io/library/busybox:latest", "", "", writer, nil, SigningOptions{}, false, false)
	assert.NoError(t, err)
	bbglibc, err := ir.New(context.Background(), "docker.io/library/busybox:glibc", "", "", writer, nil, SigningOptions{}, false, false)
	assert.NoError(t, err)

	tm, err := makeLocalMatrix(bb, bbglibc)
	assert.NoError(t, err)

	for _, image := range tm {
		// tag our images
		image.img.TagImage(image.taggedName)
		assert.NoError(t, err)
		for _, name := range image.names {
			newImage, err := ir.NewFromLocal(name)
			assert.NoError(t, err)
			assert.Equal(t, newImage.ID(), image.img.ID())
		}
	}

	// Shutdown the runtime and remove the temporary storage
	cleanup(workdir, ir)
}

// TestImage_New tests pulling the image by various names, tags, and from
// different registries
func TestImage_New(t *testing.T) {
	var names []string
	workdir, err := mkWorkDir()
	assert.NoError(t, err)

	so := storage.StoreOptions{
		RunRoot:   workdir,
		GraphRoot: workdir,
	}
	ir, err := NewImageRuntimeFromOptions(so)
	assert.NoError(t, err)
	// Build the list of pull names
	names = append(names, bbNames...)
	names = append(names, fedoraNames...)
	var writer io.Writer
	writer = os.Stdout

	// Iterate over the names and delete the image
	// after the pull
	for _, img := range names {
		newImage, err := ir.New(context.Background(), img, "", "", writer, nil, SigningOptions{}, false, false)
		assert.NoError(t, err)
		assert.NotEqual(t, newImage.ID(), "")
		err = newImage.Remove(false)
		assert.NoError(t, err)
	}

	// Shutdown the runtime and remove the temporary storage
	cleanup(workdir, ir)
}

// TestImage_MatchRepoTag tests the various inputs we need to match
// against an image's reponames
func TestImage_MatchRepoTag(t *testing.T) {
	//Set up
	workdir, err := mkWorkDir()
	assert.NoError(t, err)

	so := storage.StoreOptions{
		RunRoot:   workdir,
		GraphRoot: workdir,
	}
	ir, err := NewImageRuntimeFromOptions(so)
	assert.NoError(t, err)
	newImage, err := ir.New(context.Background(), "busybox", "", "", os.Stdout, nil, SigningOptions{}, false, false)
	assert.NoError(t, err)
	err = newImage.TagImage("foo:latest")
	assert.NoError(t, err)
	err = newImage.TagImage("foo:bar")
	assert.NoError(t, err)

	// Tests start here.
	for _, name := range bbNames {
		repoTag, err := newImage.MatchRepoTag(name)
		assert.NoError(t, err)
		assert.Equal(t, "docker.io/library/busybox:latest", repoTag)
	}

	// Test against tagged images of busybox

	// foo should resolve to foo:latest
	repoTag, err := newImage.MatchRepoTag("foo")
	assert.NoError(t, err)
	assert.Equal(t, "localhost/foo:latest", repoTag)

	// foo:bar should resolve to foo:bar
	repoTag, err = newImage.MatchRepoTag("foo:bar")
	assert.NoError(t, err)
	assert.Equal(t, "localhost/foo:bar", repoTag)
	// Shutdown the runtime and remove the temporary storage
	cleanup(workdir, ir)
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

func TestNormalizeTag(t *testing.T) {
	const digestSuffix = "@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	for _, c := range []struct{ input, expected string }{
		{"#", ""}, // Clearly invalid
		{"example.com/busybox", "example.com/busybox:latest"},                                            // Qualified name-only
		{"example.com/busybox:notlatest", "example.com/busybox:notlatest"},                               // Qualified name:tag
		{"example.com/busybox" + digestSuffix, "example.com/busybox" + digestSuffix + ":none"},           // Qualified name@digest; FIXME: The result is not even syntactically valid!
		{"example.com/busybox:notlatest" + digestSuffix, "example.com/busybox:notlatest" + digestSuffix}, // Qualified name:tag@digest
		{"busybox:latest", "localhost/busybox:latest"},                                                   // Unqualified name-only
		{"ns/busybox:latest", "ns/busybox:latest"},                                                       // Unqualified with a dot-less namespace FIXME: "ns" is treated as a registry
	} {
		res, err := normalizeTag(c.input)
		if c.expected == "" {
			assert.Error(t, err, c.input)
		} else {
			assert.NoError(t, err, c.input)
			assert.Equal(t, c.expected, res)
		}
	}
}
