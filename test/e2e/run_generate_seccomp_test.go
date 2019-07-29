package integration

import (
	"io/ioutil"
	"os"
	"time"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman generate profile", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		SkipIfRootless()
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
		podmanTest.SeedImages()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)
	})
	FIt("podman generate-seccomp generates profile that works ", func() {
		tmpfile, _ := ioutil.TempFile("", "generate-seccomp.*.json")
		fileName := tmpfile.Name()
		tmpfile.Close()
		defer os.Remove(fileName)
		time.Sleep(time.Second * 2)
		session := podmanTest.Podman([]string{"run", "--generate-seccomp=" + fileName, ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		result := podmanTest.Podman([]string{"run", "--security-opt", "seccomp=" + fileName, ALPINE, "true"})
		result.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})
	FIt("podman generate-seccomp should fail to run command barred in logfile", func() {
		tmpfile, _ := ioutil.TempFile("", "generate-seccomp.*.json")
		fileName := tmpfile.Name()
		tmpfile.Close()
		defer os.Remove(fileName)
		time.Sleep(time.Second * 2)
		session := podmanTest.Podman([]string{"run", "--generate-seccomp=" + fileName, ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		result := podmanTest.Podman([]string{"run", "--security-opt", "seccomp=" + fileName, ALPINE, "sync"})
		result.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})
})
