package integration

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
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
		Expect(session).Should(Exit(0))
	})

	It("podman info --format GO template", func() {
		session := podmanTest.Podman([]string{"info", "--format", "{{.Registries}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("registry"))
	})

	It("podman info --format GO template plugins", func() {
		session := podmanTest.Podman([]string{"info", "--format", "{{.Plugins}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("local"))
		Expect(session.OutputToString()).To(ContainSubstring("journald"))
		Expect(session.OutputToString()).To(ContainSubstring("bridge"))
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
		Expect(err).ToNot(HaveOccurred())

		err = os.MkdirAll(filepath.Dir(configPath), os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		rootlessStoragePath := `"/tmp/$HOME/$USER/$UID/storage"`
		driver := `"overlay"`
		storageOpt := `"/usr/bin/fuse-overlayfs"`
		storageConf := []byte(fmt.Sprintf("[storage]\ndriver=%s\nrootless_storage_path=%s\n[storage.options]\nmount_program=%s", driver, rootlessStoragePath, storageOpt))
		err = os.WriteFile(configPath, storageConf, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		u, err := user.Current()
		Expect(err).ToNot(HaveOccurred())

		// Cannot use podmanTest.Podman() and test for storage path
		expect := filepath.Join("/tmp", os.Getenv("HOME"), u.Username, u.Uid, "storage")
		podmanPath := podmanTest.PodmanTest.PodmanBinary
		cmd := exec.Command(podmanPath, "info", "--format", "{{.Store.GraphRoot -}}")
		out, err := cmd.CombinedOutput()
		Expect(err).ToNot(HaveOccurred())
		Expect(string(out)).To(Equal(expect))
	})

	It("check RemoteSocket ", func() {
		session := podmanTest.Podman([]string{"info", "--format", "{{.Host.RemoteSocket.Path}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(MatchRegexp("/run/.*podman.*sock"))

		session = podmanTest.Podman([]string{"info", "--format", "{{.Host.ServiceIsRemote}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		if podmanTest.RemoteTest {
			Expect(session.OutputToString()).To(Equal("true"))
		} else {
			Expect(session.OutputToString()).To(Equal("false"))
		}

		session = podmanTest.Podman([]string{"info", "--format", "{{.Host.RemoteSocket.Exists}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		if IsRemote() {
			Expect(session.OutputToString()).To(ContainSubstring("true"))
		}

	})

	It("Podman info must contain cgroupControllers with RelevantControllers", func() {
		SkipIfRootless("Hard to tell which controllers are going to be enabled for rootless")
		SkipIfRootlessCgroupsV1("Disable cgroups not supported on cgroupv1 for rootless users")
		session := podmanTest.Podman([]string{"info", "--format", "{{.Host.CgroupControllers}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("memory"))
		Expect(session.OutputToString()).To(ContainSubstring("pids"))
	})

	It("Podman info: check desired runtime", func() {
		// defined in .cirrus.yml
		want := os.Getenv("CI_DESIRED_RUNTIME")
		if want == "" {
			if os.Getenv("CIRRUS_CI") == "" {
				Skip("CI_DESIRED_RUNTIME is not set--this is OK because we're not running under Cirrus")
			}
			Fail("CIRRUS_CI is set, but CI_DESIRED_RUNTIME is not! See #14912")
		}
		session := podmanTest.Podman([]string{"info", "--format", "{{.Host.OCIRuntime.Name}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(Exit(0))
		Expect(session.OutputToString()).To(Equal(want))
	})

	It("Podman info: check desired network backend", func() {
		// defined in .cirrus.yml
		want := os.Getenv("CI_DESIRED_NETWORK")
		if want == "" {
			if os.Getenv("CIRRUS_CI") == "" {
				Skip("CI_DESIRED_NETWORK is not set--this is OK because we're not running under Cirrus")
			}
			Fail("CIRRUS_CI is set, but CI_DESIRED_NETWORK is not! See #16389")
		}
		session := podmanTest.Podman([]string{"info", "--format", "{{.Host.NetworkBackend}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(Exit(0))
		Expect(session.OutputToString()).To(Equal(want))
	})

	It("Podman info: check desired database backend", func() {
		// defined in .cirrus.yml
		want := os.Getenv("CI_DESIRED_DATABASE")
		if want == "" {
			if os.Getenv("CIRRUS_CI") == "" {
				Skip("CI_DESIRED_DATABASE is not set--this is OK because we're not running under Cirrus")
			}
			Fail("CIRRUS_CI is set, but CI_DESIRED_DATABASE is not! See #16389")
		}
		session := podmanTest.Podman([]string{"info", "--format", "{{.Host.DatabaseBackend}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(Exit(0))
		Expect(session.OutputToString()).To(Equal(want))
	})
})
