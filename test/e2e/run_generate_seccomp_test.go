package integration

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"reflect"
	"time"

	"github.com/docker/docker/api/types"

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
	It("podman generate-seccomp generates profile that works ", func() {
		tmpfile, _ := ioutil.TempFile("", "generate-seccomp.*.json")
		fileName := tmpfile.Name()

		defer tmpfile.Close()
		defer os.Remove(fileName)

		session := podmanTest.Podman([]string{"run", "--annotation", "io.podman.trace-syscall=" + fileName, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		time.Sleep(time.Second * 2)

		result := podmanTest.Podman([]string{"run", "--security-opt", "seccomp=" + fileName, ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})
	It("podman generate-seccomp should fail to run syscalls not present in the profile", func() {
		tmpfile, _ := ioutil.TempFile("", "generate-seccomp.*.json")
		fileName := tmpfile.Name()

		defer tmpfile.Close()
		defer os.Remove(fileName)

		session := podmanTest.Podman([]string{"run", "--annotation", "io.podman.trace-syscall=" + fileName, ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		time.Sleep(time.Second * 2)

		result := podmanTest.Podman([]string{"run", "--security-opt", "seccomp=" + fileName, ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).ToNot(Equal(0))
	})
	It("podman generate-seccomp should generate the same seccomp profile for identical containers", func() {
		tmpfile, _ := ioutil.TempFile("", "generate-seccomp.*.json")
		profilePath1 := tmpfile.Name()

		tmpfile.Close()
		defer os.Remove(profilePath1)

		session := podmanTest.Podman([]string{"run", "--annotation", "io.podman.trace-syscall=" + profilePath1, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		tmpfile, _ = ioutil.TempFile("", "generate-seccomp.*.json")
		profilePath2 := tmpfile.Name()

		tmpfile.Close()
		defer os.Remove(profilePath2)

		result := podmanTest.Podman([]string{"run", "--annotation", "io.podman.trace-syscall=" + profilePath2, ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))

		time.Sleep(time.Second * 2)

		profileJSON1, _ := ioutil.ReadFile(profilePath1)
		profile1 := types.Seccomp{}
		err := json.Unmarshal(profileJSON1, &profile1)
		Expect(err).To(BeNil())

		profileJSON2, _ := ioutil.ReadFile(profilePath2)
		profile2 := types.Seccomp{}
		err = json.Unmarshal(profileJSON2, &profile2)
		Expect(err).To(BeNil())
		Expect(reflect.DeepEqual(profile1.Syscalls, profile1.Syscalls)).To(Equal(true))
	})
})
