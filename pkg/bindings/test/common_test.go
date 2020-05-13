package test_bindings

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/containers/libpod/libpod/define"
	. "github.com/containers/libpod/pkg/bindings"
	"github.com/containers/libpod/pkg/bindings/containers"
	"github.com/containers/libpod/pkg/specgen"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega/gexec"
	"github.com/pkg/errors"
)

type testImage struct {
	name        string
	shortName   string
	tarballName string
}

const (
	devPodmanBinaryLocation     string = "../../../bin/podman"
	defaultPodmanBinaryLocation string = "/usr/bin/podman"
)

func getPodmanBinary() string {
	_, err := os.Stat(devPodmanBinaryLocation)
	if os.IsNotExist(err) {
		return defaultPodmanBinaryLocation
	}
	return devPodmanBinaryLocation
}

var (
	ImageCacheDir = "/tmp/podman/imagecachedir"
	LockTmpDir    string
	alpine        = testImage{
		name:        "docker.io/library/alpine:latest",
		shortName:   "alpine",
		tarballName: "alpine.tar",
	}
	busybox = testImage{
		name:        "docker.io/library/busybox:latest",
		shortName:   "busybox",
		tarballName: "busybox.tar",
	}
	CACHE_IMAGES = []testImage{alpine, busybox}
)

type bindingTest struct {
	artifactDirPath string
	imageCacheDir   string
	sock            string
	tempDirPath     string
	runRoot         string
	crioRoot        string
	conn            context.Context
}

func (b *bindingTest) NewConnection() error {
	connText, err := NewConnection(context.Background(), b.sock)
	if err != nil {
		return err
	}
	b.conn = connText
	return nil
}

func (b *bindingTest) runPodman(command []string) *gexec.Session {
	var cmd []string
	podmanBinary := getPodmanBinary()
	val, ok := os.LookupEnv("PODMAN_BINARY")
	if ok {
		podmanBinary = val
	}
	val, ok = os.LookupEnv("CGROUP_MANAGER")
	if ok {
		cmd = append(cmd, "--cgroup-manager", val)
	}
	val, ok = os.LookupEnv("CNI_CONFIG_DIR")
	if ok {
		cmd = append(cmd, "--cni-config-dir", val)
	}
	val, ok = os.LookupEnv("CONMON")
	if ok {
		cmd = append(cmd, "--conmon", val)
	}
	val, ok = os.LookupEnv("ROOT")
	if ok {
		cmd = append(cmd, "--root", val)
	} else {
		cmd = append(cmd, "--root", b.crioRoot)
	}
	val, ok = os.LookupEnv("OCI_RUNTIME")
	if ok {
		cmd = append(cmd, "--runtime", val)
	}
	val, ok = os.LookupEnv("RUNROOT")
	if ok {
		cmd = append(cmd, "--runroot", val)
	} else {
		cmd = append(cmd, "--runroot", b.runRoot)
	}
	val, ok = os.LookupEnv("TEMPDIR")
	if ok {
		cmd = append(cmd, "--tmpdir", val)
	} else {
		cmd = append(cmd, "--tmpdir", b.tempDirPath)
	}
	val, ok = os.LookupEnv("STORAGE_DRIVER")
	if ok {
		cmd = append(cmd, "--storage-driver", val)
	}
	val, ok = os.LookupEnv("STORAGE_OPTIONS")
	if ok {
		cmd = append(cmd, "--storage", val)
	}
	cmd = append(cmd, command...)
	c := exec.Command(podmanBinary, cmd...)
	fmt.Printf("Running: %s %s\n", podmanBinary, strings.Join(cmd, " "))
	session, err := gexec.Start(c, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	if err != nil {
		panic(errors.Errorf("unable to run podman command: %q", cmd))
	}
	return session
}

func newBindingTest() *bindingTest {
	tmpPath, _ := createTempDirInTempDir()
	b := bindingTest{
		crioRoot:        filepath.Join(tmpPath, "crio"),
		runRoot:         filepath.Join(tmpPath, "run"),
		artifactDirPath: "",
		imageCacheDir:   "",
		sock:            fmt.Sprintf("unix://%s", filepath.Join(tmpPath, "api.sock")),
		tempDirPath:     tmpPath,
	}
	return &b
}

// createTempDirinTempDir create a temp dir with prefix podman_test
func createTempDirInTempDir() (string, error) {
	return ioutil.TempDir("", "libpod_api")
}

func (b *bindingTest) startAPIService() *gexec.Session {
	var (
		cmd []string
	)
	cmd = append(cmd, "--log-level=debug", "--events-backend=file", "system", "service", "--timeout=0", b.sock)
	return b.runPodman(cmd)
}

func (b *bindingTest) cleanup() {
	s := b.runPodman([]string{"stop", "-a", "-t", "0"})
	s.Wait(45)
	if err := os.RemoveAll(b.tempDirPath); err != nil {
		fmt.Println(err)
	}
}

// Pull is a helper function to pull in images
func (b *bindingTest) Pull(name string) {
	p := b.runPodman([]string{"pull", name})
	p.Wait(45)
}

func (b *bindingTest) Save(i testImage) {
	p := b.runPodman([]string{"save", "-o", filepath.Join(ImageCacheDir, i.tarballName), i.name})
	p.Wait(45)
}

func (b *bindingTest) RestoreImagesFromCache() {
	for _, i := range CACHE_IMAGES {
		b.restoreImageFromCache(i)
	}
}
func (b *bindingTest) restoreImageFromCache(i testImage) {
	p := b.runPodman([]string{"load", "-i", filepath.Join(ImageCacheDir, i.tarballName), i.name})
	p.Wait(45)
}

// Run a container within or without a pod
// and add or append the alpine image to it
func (b *bindingTest) RunTopContainer(containerName *string, insidePod *bool, podName *string) (string, error) {
	s := specgen.NewSpecGenerator(alpine.name, false)
	s.Terminal = false
	s.Command = []string{"/usr/bin/top"}
	if containerName != nil {
		s.Name = *containerName
	}
	if insidePod != nil && podName != nil {
		s.Pod = *podName
	}
	ctr, err := containers.CreateWithSpec(b.conn, s)
	if err != nil {
		return "", nil
	}
	err = containers.Start(b.conn, ctr.ID, nil)
	if err != nil {
		return "", err
	}
	wait := define.ContainerStateRunning
	_, err = containers.Wait(b.conn, ctr.ID, &wait)
	return ctr.ID, err
}

// This method creates a pod with the given pod name.
// Podname is an optional parameter
func (b *bindingTest) Podcreate(name *string) {
	if name != nil {
		podname := *name
		b.runPodman([]string{"pod", "create", "--name", podname}).Wait(45)
	} else {
		b.runPodman([]string{"pod", "create"}).Wait(45)
	}
}

//  StringInSlice returns a boolean based on whether a given
//  string is in a given slice
func StringInSlice(s string, sl []string) bool {
	for _, val := range sl {
		if s == val {
			return true
		}
	}
	return false
}

var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	// make cache dir
	if err := os.MkdirAll(ImageCacheDir, 0777); err != nil {
		fmt.Printf("%q\n", err)
		os.Exit(1)
	}

	// If running localized tests, the cache dir is created and populated. if the
	// tests are remote, this is a no-op
	createCache()
	path, err := ioutil.TempDir("", "libpodlock")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return []byte(path)
}, func(data []byte) {
	LockTmpDir = string(data)
})

func createCache() {
	b := newBindingTest()
	for _, i := range CACHE_IMAGES {
		_, err := os.Stat(filepath.Join(ImageCacheDir, i.tarballName))
		if os.IsNotExist(err) {
			//	pull the image
			b.Pull(i.name)
			b.Save(i)
		}
	}
	b.cleanup()
}

func isStopped(state string) bool {
	return state == "exited" || state == "stopped"
}
