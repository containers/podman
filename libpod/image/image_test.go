package image

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/containers/storage"
	"github.com/containers/storage/pkg/reexec"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
	"github.com/stretchr/testify/assert"
)

var (
	bbNames      = []string{"docker.io/library/busybox:latest", "docker.io/library/busybox", "docker.io/busybox:latest", "docker.io/busybox", "busybox:latest", "busybox"}
	bbGlibcNames = []string{"docker.io/library/busybox:glibc", "docker.io/busybox:glibc", "busybox:glibc"}
	fedoraNames  = []string{"registry.fedoraproject.org/fedora-minimal:latest", "registry.fedoraproject.org/fedora-minimal", "fedora-minimal:latest", "fedora-minimal"}
)

// setup a runtime for the tests in an alternative location on the filesystem
func setupRuntime(workdir string) (*libpod.Runtime, error) {
	if reexec.Init() {
		return nil, errors.Errorf("dude")
	}
	sc := libpod.WithStorageConfig(storage.StoreOptions{
		GraphRoot: workdir,
		RunRoot:   workdir,
	})
	sd := libpod.WithStaticDir(path.Join(workdir, "libpod_tmp"))
	td := libpod.WithTmpDir(path.Join(workdir, "tmpdir"))

	options := []libpod.RuntimeOption{sc, sd, td}
	return libpod.NewRuntime(options...)
}

// getImage is only used to build a test matrix for testing local images
func getImage(r *libpod.Runtime, fqImageName string) (*storage.Image, error) {
	img, err := NewFromLocal(fqImageName, r)
	if err != nil {
		return nil, err
	}
	return img.image, nil
}

func tagImage(r *libpod.Runtime, fqImageName, tagName string) error {
	img, err := NewFromLocal(fqImageName, r)
	if err != nil {
		return err
	}
	r.TagImage(img.image, tagName)
	return nil
}

type localImageTest struct {
	fqname, taggedName string
	img                *storage.Image
	names              []string
}

// make a temporary directory for the runtime
func mkWorkDir() (string, error) {
	return ioutil.TempDir("", "podman-test")
}

// shutdown the runtime and clean behind it
func cleanup(r *libpod.Runtime, workdir string) {
	r.Shutdown(true)
	err := os.RemoveAll(workdir)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func makeLocalMatrix(r *libpod.Runtime) ([]localImageTest, error) {
	var l []localImageTest
	// busybox
	busybox := localImageTest{
		fqname:     "docker.io/library/busybox:latest",
		taggedName: "bb:latest",
	}
	b, err := getImage(r, busybox.fqname)
	if err != nil {
		return nil, err
	}
	busybox.img = b
	busybox.names = bbNames
	busybox.names = append(busybox.names, []string{"bb:latest", "bb", b.ID, b.ID[0:7], fmt.Sprintf("busybox@%s", b.Digest.String())}...)

	//fedora
	fedora := localImageTest{
		fqname:     "registry.fedoraproject.org/fedora-minimal:latest",
		taggedName: "f27:latest",
	}
	f, err := getImage(r, fedora.fqname)
	if err != nil {
		return nil, err
	}
	fedora.img = f
	fedora.names = fedoraNames

	// busybox-glibc
	busyboxGlibc := localImageTest{
		fqname:     "docker.io/library/busybox:glibc",
		taggedName: "bb:glibc",
	}
	bg, err := getImage(r, busyboxGlibc.fqname)
	if err != nil {
		return nil, err
	}
	busyboxGlibc.img = bg
	busyboxGlibc.names = bbGlibcNames

	l = append(l, busybox, fedora)
	return l, nil

}

// TestImage_NewFromLocal tests finding the image locally by various names,
// tags, and aliases
func TestImage_NewFromLocal(t *testing.T) {
	workdir, err := mkWorkDir()
	assert.NoError(t, err)
	runtime, err := setupRuntime(workdir)
	assert.NoError(t, err)

	// Need images to be present for this test
	_, err = runtime.PullImage("docker.io/library/busybox:latest", libpod.CopyOptions{})
	assert.NoError(t, err)
	_, err = runtime.PullImage("docker.io/library/busybox:glibc", libpod.CopyOptions{})
	assert.NoError(t, err)
	_, err = runtime.PullImage("registry.fedoraproject.org/fedora-minimal:latest", libpod.CopyOptions{})
	assert.NoError(t, err)

	tm, err := makeLocalMatrix(runtime)
	assert.NoError(t, err)
	for _, image := range tm {
		// tag our images
		err = tagImage(runtime, image.fqname, image.taggedName)
		assert.NoError(t, err)
		for _, name := range image.names {
			newImage, err := NewFromLocal(name, runtime)
			assert.NoError(t, err)
			assert.Equal(t, newImage.ID(), image.img.ID)
		}
	}

	// Shutdown the runtime and remove the temporary storage
	cleanup(runtime, workdir)
}

// TestImage_New tests pulling the image by various names, tags, and from
// different registries
func TestImage_New(t *testing.T) {
	var names []string
	workdir, err := mkWorkDir()
	assert.NoError(t, err)
	runtime, err := setupRuntime(workdir)
	assert.NoError(t, err)

	// Build the list of pull names
	names = append(names, bbNames...)
	names = append(names, fedoraNames...)

	// Iterate over the names and delete the image
	// after the pull
	for _, img := range names {
		_, err := runtime.GetImage(img)
		if err == nil {
			os.Exit(1)
		}
		newImage, err := New(img, runtime)
		assert.NoError(t, err)
		assert.NotEqual(t, newImage.ID(), "")
		err = newImage.Remove(false)
		assert.NoError(t, err)
	}

	// Shutdown the runtime and remove the temporary storage
	cleanup(runtime, workdir)
}
