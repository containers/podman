package integration

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman commit", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		Expect(err).To(BeNil())
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman commit container", func() {
		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"commit", "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		check := podmanTest.Podman([]string{"inspect", "foobar.com/test1-image:latest"})
		check.WaitWithDefaultTimeout()
		data := check.InspectImageJSON()
		Expect(data[0].RepoTags).To(ContainElement("foobar.com/test1-image:latest"))
	})

	It("podman commit single letter container", func() {
		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"commit", "test1", "a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		check := podmanTest.Podman([]string{"inspect", "localhost/a:latest"})
		check.WaitWithDefaultTimeout()
		data := check.InspectImageJSON()
		Expect(data[0].RepoTags).To(ContainElement("localhost/a:latest"))
	})

	It("podman container commit container", func() {
		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"container", "commit", "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		check := podmanTest.Podman([]string{"image", "inspect", "foobar.com/test1-image:latest"})
		check.WaitWithDefaultTimeout()
		data := check.InspectImageJSON()
		Expect(data[0].RepoTags).To(ContainElement("foobar.com/test1-image:latest"))
	})

	It("podman commit container with message", func() {
		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"commit", "-f", "docker", "--message", "testing-commit", "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		check := podmanTest.Podman([]string{"inspect", "foobar.com/test1-image:latest"})
		check.WaitWithDefaultTimeout()
		data := check.InspectImageJSON()
		Expect(data[0]).To(HaveField("Comment", "testing-commit"))
	})

	It("podman commit container with author", func() {
		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"commit", "--author", "snoopy", "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		check := podmanTest.Podman([]string{"inspect", "foobar.com/test1-image:latest"})
		check.WaitWithDefaultTimeout()
		data := check.InspectImageJSON()
		Expect(data[0]).To(HaveField("Author", "snoopy"))
	})

	It("podman commit container with change flag", func() {
		test := podmanTest.Podman([]string{"run", "--name", "test1", "-d", ALPINE, "ls"})
		test.WaitWithDefaultTimeout()
		Expect(test).Should(Exit(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"commit", "--change", "LABEL=image=blue", "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		check := podmanTest.Podman([]string{"inspect", "foobar.com/test1-image:latest"})
		check.WaitWithDefaultTimeout()
		data := check.InspectImageJSON()
		foundBlue := false
		for _, i := range data[0].Labels {
			if i == "blue" {
				foundBlue = true
				break
			}
		}
		Expect(foundBlue).To(Equal(true))
	})

	It("podman commit container with --squash", func() {
		test := podmanTest.Podman([]string{"run", "--name", "test1", "-d", ALPINE, "ls"})
		test.WaitWithDefaultTimeout()
		Expect(test).Should(Exit(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"commit", "--squash", "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RootFS.Layers}}", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		// Check for one layers
		Expect(strings.Fields(session.OutputToString())).To(HaveLen(1))
	})

	It("podman commit container with change flag and JSON entrypoint with =", func() {
		test := podmanTest.Podman([]string{"run", "--name", "test1", "-d", ALPINE, "ls"})
		test.WaitWithDefaultTimeout()
		Expect(test).Should(Exit(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"commit", "--change", `ENTRYPOINT ["foo", "bar=baz"]`, "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		check := podmanTest.Podman([]string{"inspect", "foobar.com/test1-image:latest"})
		check.WaitWithDefaultTimeout()
		data := check.InspectImageJSON()
		Expect(data).To(HaveLen(1))
		Expect(data[0].Config.Entrypoint).To(HaveLen(2))
		Expect(data[0].Config.Entrypoint[0]).To(Equal("foo"))
		Expect(data[0].Config.Entrypoint[1]).To(Equal("bar=baz"))
	})

	It("podman commit container with change CMD flag", func() {
		test := podmanTest.Podman([]string{"run", "--name", "test1", "-d", ALPINE, "ls"})
		test.WaitWithDefaultTimeout()
		Expect(test).Should(Exit(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"commit", "--change", "CMD a b c", "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.Config.Cmd}}", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("sh -c a b c"))

		session = podmanTest.Podman([]string{"commit", "--change", "CMD=[\"a\",\"b\",\"c\"]", "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.Config.Cmd}}", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Not(ContainSubstring("sh -c")))
	})

	It("podman commit container with pause flag", func() {
		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"commit", "--pause=false", "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		check := podmanTest.Podman([]string{"inspect", "foobar.com/test1-image:latest"})
		check.WaitWithDefaultTimeout()
		Expect(check).Should(Exit(0))
	})

	It("podman commit with volumes mounts and no include-volumes", func() {
		s := podmanTest.Podman([]string{"run", "--name", "test1", "-v", "/tmp:/foo", "alpine", "date"})
		s.WaitWithDefaultTimeout()
		Expect(s).Should(Exit(0))

		c := podmanTest.Podman([]string{"commit", "test1", "newimage"})
		c.WaitWithDefaultTimeout()
		Expect(c).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", "newimage"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		image := inspect.InspectImageJSON()
		Expect(image[0].Config.Volumes).To(Not(HaveKey("/foo")))
	})

	It("podman commit with volume mounts and --include-volumes", func() {
		// We need to figure out how volumes are going to work correctly with the remote
		// client.  This does not currently work.
		SkipIfRemote("--testing Remote Volumes")
		s := podmanTest.Podman([]string{"run", "--name", "test1", "-v", "/tmp:/foo", "alpine", "date"})
		s.WaitWithDefaultTimeout()
		Expect(s).Should(Exit(0))

		c := podmanTest.Podman([]string{"commit", "--include-volumes", "test1", "newimage"})
		c.WaitWithDefaultTimeout()
		Expect(c).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", "newimage"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		image := inspect.InspectImageJSON()
		Expect(image[0].Config.Volumes).To(HaveKey("/foo"))

		r := podmanTest.Podman([]string{"run", "newimage"})
		r.WaitWithDefaultTimeout()
		Expect(r).Should(Exit(0))
	})

	It("podman commit container check env variables", func() {
		s := podmanTest.Podman([]string{"run", "--name", "test1", "-e", "TEST=1=1-01=9.01", "-it", "alpine", "true"})
		s.WaitWithDefaultTimeout()
		Expect(s).Should(Exit(0))

		c := podmanTest.Podman([]string{"commit", "test1", "newimage"})
		c.WaitWithDefaultTimeout()
		Expect(c).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", "newimage"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		image := inspect.InspectImageJSON()

		envMap := make(map[string]bool)
		for _, v := range image[0].Config.Env {
			envMap[v] = true
		}
		Expect(envMap).To(HaveKey("TEST=1=1-01=9.01"))
	})

	It("podman commit container and print id to external file", func() {
		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).To(BeNil())
		Expect(os.Chdir(os.TempDir())).To(BeNil())
		targetPath, err := CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		targetFile := filepath.Join(targetPath, "idFile")
		defer Expect(os.RemoveAll(targetFile)).To(BeNil())
		defer Expect(os.Chdir(cwd)).To(BeNil())

		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"commit", "test1", "foobar.com/test1-image:latest", "--iidfile", targetFile})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		id, _ := os.ReadFile(targetFile)
		check := podmanTest.Podman([]string{"inspect", "foobar.com/test1-image:latest"})
		check.WaitWithDefaultTimeout()
		data := check.InspectImageJSON()
		Expect(data[0]).To(HaveField("ID", string(id)))
	})

	It("podman commit should not commit secret", func() {
		secretsString := "somesecretdata"
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte(secretsString), 0755)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"secret", "create", "mysecret", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "--secret", "mysecret", "--name", "secr", ALPINE, "cat", "/run/secrets/mysecret"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal(secretsString))

		session = podmanTest.Podman([]string{"commit", "secr", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "foobar.com/test1-image:latest", "cat", "/run/secrets/mysecret"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

	})

	It("podman commit should not commit env secret", func() {
		secretsString := "somesecretdata"
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte(secretsString), 0755)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"secret", "create", "mysecret", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "--secret", "source=mysecret,type=env", "--name", "secr", ALPINE, "printenv", "mysecret"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal(secretsString))

		session = podmanTest.Podman([]string{"commit", "secr", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"run", "foobar.com/test1-image:latest", "printenv", "mysecret"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(Not(ContainSubstring(secretsString)))
	})

	It("podman commit adds exposed ports", func() {
		name := "testcon"
		s := podmanTest.Podman([]string{"run", "--name", name, "-p", "8585:80", ALPINE, "true"})
		s.WaitWithDefaultTimeout()
		Expect(s).Should(Exit(0))

		newImageName := "newimage"
		c := podmanTest.Podman([]string{"commit", name, newImageName})
		c.WaitWithDefaultTimeout()
		Expect(c).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"inspect", newImageName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		images := inspect.InspectImageJSON()
		Expect(images).To(HaveLen(1))
		Expect(images[0].Config.ExposedPorts).To(HaveKey("80/tcp"))

		name = "testcon2"
		s = podmanTest.Podman([]string{"run", "--name", name, "-d", NGINX_IMAGE})
		s.WaitWithDefaultTimeout()
		Expect(s).Should(Exit(0))

		newImageName = "newimage2"
		c = podmanTest.Podman([]string{"commit", name, newImageName})
		c.WaitWithDefaultTimeout()
		Expect(c).Should(Exit(0))

		inspect = podmanTest.Podman([]string{"inspect", newImageName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		images = inspect.InspectImageJSON()
		Expect(images).To(HaveLen(1))
		Expect(images[0].Config.ExposedPorts).To(HaveKey("80/tcp"))
	})
})
