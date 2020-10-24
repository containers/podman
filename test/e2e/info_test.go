package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	. "github.com/containers/podman/v2/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
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

	It("podman info --format json", func() {
		tests := []struct {
			input    string
			success  bool
			exitCode int
		}{
			{"json", true, 0},
			{" json", true, 0},
			{"json ", true, 0},
			{"  json   ", true, 0},
			{"{{json .}}", true, 0},
			{"{{ json .}}", true, 0},
			{"{{json .   }}", true, 0},
			{"  {{  json .    }}   ", true, 0},
			{"{{json }}", true, 0},
			{"{{json .", false, 125},
			{"json . }}", false, 0}, // without opening {{ template seen as string literal
		}
		for _, tt := range tests {
			session := podmanTest.Podman([]string{"info", "--format", tt.input})
			session.WaitWithDefaultTimeout()

			desc := fmt.Sprintf("JSON test(%q)", tt.input)
			Expect(session).Should(Exit(tt.exitCode), desc)
			Expect(session.IsJSONOutputValid()).To(Equal(tt.success), desc)
		}
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
		SkipIfNotRootless("test of rootless_storage_path is only meaningful as rootless")
		SkipIfRemote("Only tests storage on local client")
		configPath := filepath.Join(podmanTest.TempDir, ".config", "containers", "storage.conf")
		os.Setenv("CONTAINERS_STORAGE_CONF", configPath)
		defer func() {
			os.Unsetenv("CONTAINERS_STORAGE_CONF")
		}()
		err := os.RemoveAll(filepath.Dir(configPath))
		Expect(err).To(BeNil())

		err = os.MkdirAll(filepath.Dir(configPath), os.ModePerm)
		Expect(err).To(BeNil())

		rootlessStoragePath := `"/tmp/$HOME/$USER/$UID/storage"`
		driver := `"overlay"`
		storageOpt := `"/usr/bin/fuse-overlayfs"`
		storageConf := []byte(fmt.Sprintf("[storage]\ndriver=%s\nrootless_storage_path=%s\n[storage.options]\nmount_program=%s", driver, rootlessStoragePath, storageOpt))
		err = ioutil.WriteFile(configPath, storageConf, os.ModePerm)
		Expect(err).To(BeNil())

		u, err := user.Current()
		Expect(err).To(BeNil())

		expect := filepath.Join("/tmp", os.Getenv("HOME"), u.Username, u.Uid, "storage")
		podmanPath := podmanTest.PodmanTest.PodmanBinary
		cmd := exec.Command(podmanPath, "info", "--format", "{{.Store.GraphRoot}}")
		out, err := cmd.CombinedOutput()
		fmt.Println(string(out))
		Expect(err).To(BeNil())
		Expect(string(out)).To(Equal(expect))
	})
})
