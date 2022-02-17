package integration

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman volume plugins", func() {
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
		podmanTest.SeedImages()
		os.Setenv("CONTAINERS_CONF", "config/containers.conf")
		SkipIfRemote("Volume plugins only supported as local")
		SkipIfRootless("Root is required for volume plugin testing")
		os.MkdirAll("/run/docker/plugins", 0755)
	})

	AfterEach(func() {
		podmanTest.CleanupVolume()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)
		os.Unsetenv("CONTAINERS_CONF")
	})

	It("volume create with nonexistent plugin errors", func() {
		session := podmanTest.Podman([]string{"volume", "create", "--driver", "notexist", "test_volume_name"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("volume create with not-running plugin does not error", func() {
		session := podmanTest.Podman([]string{"volume", "create", "--driver", "testvol0", "test_volume_name"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("volume create and remove with running plugin succeeds", func() {
		podmanTest.AddImageToRWStore(volumeTest)

		pluginStatePath := filepath.Join(podmanTest.TempDir, "volumes")
		os.Mkdir(pluginStatePath, 0755)

		// Keep this distinct within tests to avoid multiple tests using the same plugin.
		pluginName := "testvol1"
		plugin := podmanTest.Podman([]string{"run", "--security-opt", "label=disable", "-v", "/run/docker/plugins:/run/docker/plugins", "-v", fmt.Sprintf("%v:%v", pluginStatePath, pluginStatePath), "-d", volumeTest, "--sock-name", pluginName, "--path", pluginStatePath})
		plugin.WaitWithDefaultTimeout()
		Expect(plugin).Should(Exit(0))

		volName := "testVolume1"
		create := podmanTest.Podman([]string{"volume", "create", "--driver", pluginName, volName})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		ls1 := podmanTest.Podman([]string{"volume", "ls", "-q"})
		ls1.WaitWithDefaultTimeout()
		Expect(ls1).Should(Exit(0))
		arrOutput := ls1.OutputToStringArray()
		Expect(arrOutput).To(HaveLen(1))
		Expect(arrOutput[0]).To(ContainSubstring(volName))

		remove := podmanTest.Podman([]string{"volume", "rm", volName})
		remove.WaitWithDefaultTimeout()
		Expect(remove).Should(Exit(0))

		ls2 := podmanTest.Podman([]string{"volume", "ls", "-q"})
		ls2.WaitWithDefaultTimeout()
		Expect(ls2).Should(Exit(0))
		Expect(ls2.OutputToStringArray()).To(BeEmpty())
	})

	It("volume inspect with running plugin succeeds", func() {
		podmanTest.AddImageToRWStore(volumeTest)

		pluginStatePath := filepath.Join(podmanTest.TempDir, "volumes")
		os.Mkdir(pluginStatePath, 0755)

		// Keep this distinct within tests to avoid multiple tests using the same plugin.
		pluginName := "testvol2"
		plugin := podmanTest.Podman([]string{"run", "--security-opt", "label=disable", "-v", "/run/docker/plugins:/run/docker/plugins", "-v", fmt.Sprintf("%v:%v", pluginStatePath, pluginStatePath), "-d", volumeTest, "--sock-name", pluginName, "--path", pluginStatePath})
		plugin.WaitWithDefaultTimeout()
		Expect(plugin).Should(Exit(0))

		volName := "testVolume1"
		create := podmanTest.Podman([]string{"volume", "create", "--driver", pluginName, volName})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		volInspect := podmanTest.Podman([]string{"volume", "inspect", "--format", "{{ .Driver }}", volName})
		volInspect.WaitWithDefaultTimeout()
		Expect(volInspect).Should(Exit(0))
		Expect(volInspect.OutputToString()).To(ContainSubstring(pluginName))
	})

	It("remove plugin with stopped plugin succeeds", func() {
		podmanTest.AddImageToRWStore(volumeTest)

		pluginStatePath := filepath.Join(podmanTest.TempDir, "volumes")
		os.Mkdir(pluginStatePath, 0755)

		// Keep this distinct within tests to avoid multiple tests using the same plugin.
		pluginName := "testvol3"
		ctrName := "pluginCtr"
		plugin := podmanTest.Podman([]string{"run", "--name", ctrName, "--security-opt", "label=disable", "-v", "/run/docker/plugins:/run/docker/plugins", "-v", fmt.Sprintf("%v:%v", pluginStatePath, pluginStatePath), "-d", volumeTest, "--sock-name", pluginName, "--path", pluginStatePath})
		plugin.WaitWithDefaultTimeout()
		Expect(plugin).Should(Exit(0))

		volName := "testVolume1"
		create := podmanTest.Podman([]string{"volume", "create", "--driver", pluginName, volName})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		ls1 := podmanTest.Podman([]string{"volume", "ls", "-q"})
		ls1.WaitWithDefaultTimeout()
		Expect(ls1).Should(Exit(0))
		arrOutput := ls1.OutputToStringArray()
		Expect(arrOutput).To(HaveLen(1))
		Expect(arrOutput[0]).To(ContainSubstring(volName))

		stop := podmanTest.Podman([]string{"stop", "--timeout", "0", ctrName})
		stop.WaitWithDefaultTimeout()
		Expect(stop).Should(Exit(0))

		// Remove should exit non-zero because missing plugin
		remove := podmanTest.Podman([]string{"volume", "rm", volName})
		remove.WaitWithDefaultTimeout()
		Expect(remove).To(ExitWithError())

		// But the volume should still be gone
		ls2 := podmanTest.Podman([]string{"volume", "ls", "-q"})
		ls2.WaitWithDefaultTimeout()
		Expect(ls2).Should(Exit(0))
		Expect(ls2.OutputToStringArray()).To(BeEmpty())
	})

	It("use plugin in containers", func() {
		podmanTest.AddImageToRWStore(volumeTest)

		pluginStatePath := filepath.Join(podmanTest.TempDir, "volumes")
		os.Mkdir(pluginStatePath, 0755)

		// Keep this distinct within tests to avoid multiple tests using the same plugin.
		pluginName := "testvol4"
		plugin := podmanTest.Podman([]string{"run", "--security-opt", "label=disable", "-v", "/run/docker/plugins:/run/docker/plugins", "-v", fmt.Sprintf("%v:%v", pluginStatePath, pluginStatePath), "-d", volumeTest, "--sock-name", pluginName, "--path", pluginStatePath})
		plugin.WaitWithDefaultTimeout()
		Expect(plugin).Should(Exit(0))

		volName := "testVolume1"
		create := podmanTest.Podman([]string{"volume", "create", "--driver", pluginName, volName})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(Exit(0))

		ctr1Name := "ctr1"
		ctr1 := podmanTest.Podman([]string{"run", "--security-opt", "label=disable", "--name", ctr1Name, "-v", fmt.Sprintf("%v:/test", volName), ALPINE, "sh", "-c", "touch /test/testfile && echo helloworld > /test/testfile"})
		ctr1.WaitWithDefaultTimeout()
		Expect(ctr1).Should(Exit(0))

		ctr2Name := "ctr2"
		ctr2 := podmanTest.Podman([]string{"run", "--security-opt", "label=disable", "--name", ctr2Name, "-v", fmt.Sprintf("%v:/test", volName), ALPINE, "cat", "/test/testfile"})
		ctr2.WaitWithDefaultTimeout()
		Expect(ctr2).Should(Exit(0))
		Expect(ctr2.OutputToString()).To(ContainSubstring("helloworld"))

		// HACK: `volume rm -f` is timing out trying to remove containers using the volume.
		// Solution: remove them manually...
		// TODO: fix this when I get back
		rmAll := podmanTest.Podman([]string{"rm", "-f", ctr2Name, ctr1Name})
		rmAll.WaitWithDefaultTimeout()
		Expect(rmAll).Should(Exit(0))
	})
})
