package endpoint

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestEndpoint(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Endpoint Suite")
}

var LockTmpDir string

var _ = SynchronizedBeforeSuite(func() []byte {
	// Cache images
	cwd, _ := os.Getwd()
	INTEGRATION_ROOT = filepath.Join(cwd, "../../")
	podman := Setup("/tmp")
	podman.ArtifactPath = ARTIFACT_DIR
	if _, err := os.Stat(ARTIFACT_DIR); os.IsNotExist(err) {
		if err = os.Mkdir(ARTIFACT_DIR, 0777); err != nil {
			fmt.Printf("%q\n", err)
			os.Exit(1)
		}
	}

	// make cache dir
	if err := os.MkdirAll(ImageCacheDir, 0777); err != nil {
		fmt.Printf("%q\n", err)
		os.Exit(1)
	}

	podman.StartVarlink()
	for _, image := range CACHE_IMAGES {
		podman.createArtifact(image)
	}
	podman.StopVarlink()
	// If running localized tests, the cache dir is created and populated. if the
	// tests are remote, this is a no-op
	populateCache(podman)

	path, err := ioutil.TempDir("", "libpodlock")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return []byte(path)
}, func(data []byte) {
	LockTmpDir = string(data)
})

var _ = SynchronizedAfterSuite(func() {},
	func() {
		podman := Setup("/tmp")
		if err := os.RemoveAll(podman.CrioRoot); err != nil {
			fmt.Printf("%q\n", err)
			os.Exit(1)
		}
		if err := os.RemoveAll(podman.ImageCacheDir); err != nil {
			fmt.Printf("%q\n", err)
			os.Exit(1)
		}
	})
