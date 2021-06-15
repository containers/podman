package integration

import (
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman secret", func() {
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
	})

	AfterEach(func() {
		podmanTest.CleanupSecrets()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman secret create", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := ioutil.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"secret", "create", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID := session.OutputToString()
		Expect(session.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"secret", "inspect", "--format", "{{.ID}}", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.OutputToString()).To(Equal(secrID))
	})

	It("podman secret create bad name should fail", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := ioutil.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"secret", "create", "?!", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))
	})

	It("podman secret inspect", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := ioutil.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"secret", "create", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID := session.OutputToString()
		Expect(session.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"secret", "inspect", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.IsJSONOutputValid()).To(BeTrue())
	})

	It("podman secret inspect with --format", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := ioutil.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"secret", "create", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID := session.OutputToString()
		Expect(session.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"secret", "inspect", "--format", "{{.ID}}", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.OutputToString()).To(Equal(secrID))
	})

	It("podman secret inspect multiple secrets", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := ioutil.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"secret", "create", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID := session.OutputToString()
		Expect(session.ExitCode()).To(Equal(0))

		session2 := podmanTest.Podman([]string{"secret", "create", "b", secretFilePath})
		session2.WaitWithDefaultTimeout()
		secrID2 := session2.OutputToString()
		Expect(session2.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"secret", "inspect", secrID, secrID2})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.IsJSONOutputValid()).To(BeTrue())
	})

	It("podman secret inspect bogus", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := ioutil.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).To(BeNil())

		inspect := podmanTest.Podman([]string{"secret", "inspect", "bogus"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Not(Equal(0)))

	})

	It("podman secret ls", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := ioutil.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"secret", "create", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		list := podmanTest.Podman([]string{"secret", "ls"})
		list.WaitWithDefaultTimeout()
		Expect(list.ExitCode()).To(Equal(0))
		Expect(len(list.OutputToStringArray())).To(Equal(2))

	})

	It("podman secret ls with Go template", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := ioutil.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"secret", "create", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		list := podmanTest.Podman([]string{"secret", "ls", "--format", "table {{.Name}}"})
		list.WaitWithDefaultTimeout()

		Expect(list.ExitCode()).To(Equal(0))
		Expect(len(list.OutputToStringArray())).To(Equal(2), list.OutputToString())
	})

	It("podman secret rm", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := ioutil.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"secret", "create", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID := session.OutputToString()
		Expect(session.ExitCode()).To(Equal(0))

		removed := podmanTest.Podman([]string{"secret", "rm", "a"})
		removed.WaitWithDefaultTimeout()
		Expect(removed.ExitCode()).To(Equal(0))
		Expect(removed.OutputToString()).To(Equal(secrID))

		session = podmanTest.Podman([]string{"secret", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(1))
	})

	It("podman secret rm --all", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := ioutil.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"secret", "create", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session = podmanTest.Podman([]string{"secret", "create", "b", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		removed := podmanTest.Podman([]string{"secret", "rm", "-a"})
		removed.WaitWithDefaultTimeout()
		Expect(removed.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"secret", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(1))
	})

	It("podman secret creates from environment variable", func() {
		// no env variable set, should fail
		session := podmanTest.Podman([]string{"secret", "create", "--env", "a", "MYENVVAR"})
		session.WaitWithDefaultTimeout()
		secrID := session.OutputToString()
		Expect(session.ExitCode()).To(Not(Equal(0)))

		os.Setenv("MYENVVAR", "somedata")
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}

		session = podmanTest.Podman([]string{"secret", "create", "--env", "a", "MYENVVAR"})
		session.WaitWithDefaultTimeout()
		secrID = session.OutputToString()
		Expect(session.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"secret", "inspect", "--format", "{{.ID}}", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.OutputToString()).To(Equal(secrID))
	})

	It("podman secret create with labels", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := ioutil.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).To(BeNil())

		session1 := podmanTest.Podman([]string{"secret", "create", "--label", "foo=bar", "labelled", secretFilePath})
		session1.WaitWithDefaultTimeout()
		secrID := session1.OutputToString()
		Expect(session1.ExitCode()).To(Equal(0))

		inspect := podmanTest.Podman([]string{"secret", "inspect", "--format", "{{.Spec.Labels.foo}}", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.OutputToString()).To(Equal("bar"))
	})

	It("podman secret ls with filters", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := ioutil.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).To(BeNil())

		session := podmanTest.Podman([]string{"secret", "create", "nolabel", secretFilePath})
		session.WaitWithDefaultTimeout()
		id := session.OutputToString()
		Expect(session.ExitCode()).To(Equal(0))

		session1 := podmanTest.Podman([]string{"secret", "create", "--label", "foo=bar", "labelled", secretFilePath})
		session1.WaitWithDefaultTimeout()
		id1 := session1.OutputToString()
		Expect(session1.ExitCode()).To(Equal(0))

		session2 := podmanTest.Podman([]string{"secret", "create", "--label", "foo=bar2", "labelled2", secretFilePath})
		session2.WaitWithDefaultTimeout()
		_ = session2.OutputToString()
		Expect(session1.ExitCode()).To(Equal(0))

		// filter label foo=bar
		inspect := podmanTest.Podman([]string{"secret", "ls", "--filter", "label=foo=bar", "--format", "{{.ID}}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.OutputToString()).To(Equal(id1))

		// filter label foo
		inspect = podmanTest.Podman([]string{"secret", "ls", "--filter", "label=foo", "--format", "{{.ID}}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(len(inspect.OutputToStringArray())).To(Equal(2))

		// filter name
		inspect = podmanTest.Podman([]string{"secret", "ls", "--filter", "name=nolabel", "--format", "{{.ID}}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.OutputToString()).To(Equal(id))

		// filter id
		idfilter := "id=" + id
		inspect = podmanTest.Podman([]string{"secret", "ls", "--filter", idfilter, "--format", "{{.Name}}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(inspect.OutputToString()).To(Equal("nolabel"))

		// filter driver
		inspect = podmanTest.Podman([]string{"secret", "ls", "--filter", "driver=file", "--format", "{{.ID}}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(len(inspect.OutputToStringArray())).To(Equal(3))

		// filter should be inclusive
		inspect = podmanTest.Podman([]string{"secret", "ls", "--filter", "name=nolabel", "--filter", "name=labelled", "--format", "{{.ID}}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.ExitCode()).To(Equal(0))
		Expect(len(inspect.OutputToStringArray())).To(Equal(2))

	})

})
