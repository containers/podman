package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/containers/libpod/pkg/rootless"
	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman Info", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)
	})

	It("podman info json output", func() {
		session := podmanTest.Podman([]string{"info", "--format=json"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

	})

	It("podman info --format GO template", func() {
		session := podmanTest.Podman([]string{"info", "--format", "{{.Store.GraphRoot}}"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman info --format GO template", func() {
		session := podmanTest.Podman([]string{"info", "--format", "{{.Registries}}"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("registry"))
	})

	It("podman info rootless storage path", func() {
		if !rootless.IsRootless() {
			Skip("test of rootless_storage_path is only meaningful as rootless")
		}
		SkipIfRemote()
		oldHOME, hasHOME := os.LookupEnv("HOME")
		defer func() {
			if hasHOME {
				os.Setenv("HOME", oldHOME)
			} else {
				os.Unsetenv("HOME")
			}
		}()
		os.Setenv("HOME", podmanTest.TempDir)
		configPath := filepath.Join(os.Getenv("HOME"), ".config", "containers", "storage.conf")
		err := os.RemoveAll(filepath.Dir(configPath))
		Expect(err).To(BeNil())

		err = os.MkdirAll(filepath.Dir(configPath), os.ModePerm)
		Expect(err).To(BeNil())

		rootlessStoragePath := `"/tmp/$HOME/$USER/$UID"`
		driver := `"overlay"`
		storageOpt := `"/usr/bin/fuse-overlayfs"`
		storageConf := []byte(fmt.Sprintf("[storage]\ndriver=%s\nrootless_storage_path=%s\n[storage.options]\nmount_program=%s", driver, rootlessStoragePath, storageOpt))
		err = ioutil.WriteFile(configPath, storageConf, os.ModePerm)
		Expect(err).To(BeNil())

		expect := filepath.Join("/tmp", os.Getenv("HOME"), os.Getenv("USER"), os.Getenv("UID"))
		podmanPath := podmanTest.PodmanTest.PodmanBinary
		cmd := exec.Command(podmanPath, "info", "--format", "{{.Store.GraphRoot}}")
		out, err := cmd.CombinedOutput()
		fmt.Println(string(out))
		Expect(err).To(BeNil())
		Expect(string(out)).To(ContainSubstring(expect))
	})
})
