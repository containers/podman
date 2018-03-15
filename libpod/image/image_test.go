package image

import (
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
	bb, err := ir.New("docker.io/library/busybox:latest", "", "", writer, nil, SigningOptions{})
	assert.NoError(t, err)
	bbglibc, err := ir.New("docker.io/library/busybox:glibc", "", "", writer, nil, SigningOptions{})
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
		newImage, err := ir.New(img, "", "", writer, nil, SigningOptions{})
		assert.NoError(t, err)
		assert.NotEqual(t, newImage.ID(), "")
		err = newImage.Remove(false)
		assert.NoError(t, err)
	}

	// Shutdown the runtime and remove the temporary storage
	cleanup(workdir, ir)
}
