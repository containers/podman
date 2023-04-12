package integration

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
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
	})

	AfterEach(func() {
		podmanTest.CleanupSecrets()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("podman secret create", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "-d", "file", "--driver-opts", "opt1=val", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID := session.OutputToString()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"secret", "inspect", "--format", "{{.ID}}", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(Equal(secrID))
		inspect = podmanTest.Podman([]string{"secret", "inspect", "-f", "{{.Spec.Driver.Options}}", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring("opt1:val"))
	})

	It("podman secret create bad name should fail", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "?!", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman secret inspect", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID := session.OutputToString()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"secret", "inspect", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(BeValidJSON())
	})

	It("podman secret inspect with --format", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID := session.OutputToString()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"secret", "inspect", "--format", "{{.ID}}", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(Equal(secrID))
	})

	It("podman secret inspect with --pretty", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID := session.OutputToString()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"secret", "inspect", "--pretty", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring("Name:"))
		Expect(inspect.OutputToString()).To(ContainSubstring(secrID))
	})

	It("podman secret inspect multiple secrets", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID := session.OutputToString()
		Expect(session).Should(Exit(0))

		session2 := podmanTest.Podman([]string{"secret", "create", "b", secretFilePath})
		session2.WaitWithDefaultTimeout()
		secrID2 := session2.OutputToString()
		Expect(session2).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"secret", "inspect", secrID, secrID2})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(BeValidJSON())
	})

	It("podman secret inspect bogus", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).ToNot(HaveOccurred())

		inspect := podmanTest.Podman([]string{"secret", "inspect", "bogus"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).To(ExitWithError())
	})

	It("podman secret ls", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		list := podmanTest.Podman([]string{"secret", "ls"})
		list.WaitWithDefaultTimeout()
		Expect(list).Should(Exit(0))
		Expect(list.OutputToStringArray()).To(HaveLen(2))

	})

	It("podman secret ls --quiet", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).ToNot(HaveOccurred())

		secretName := "a"

		session := podmanTest.Podman([]string{"secret", "create", secretName, secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		secretID := session.OutputToString()

		list := podmanTest.Podman([]string{"secret", "ls", "-q"})
		list.WaitWithDefaultTimeout()
		Expect(list).Should(Exit(0))
		Expect(list.OutputToString()).To(Equal(secretID))

		list = podmanTest.Podman([]string{"secret", "ls", "--quiet"})
		list.WaitWithDefaultTimeout()
		Expect(list).Should(Exit(0))
		Expect(list.OutputToString()).To(Equal(secretID))

		// Prefer format over quiet
		list = podmanTest.Podman([]string{"secret", "ls", "-q", "--format", "{{.Name}}"})
		list.WaitWithDefaultTimeout()
		Expect(list).Should(Exit(0))
		Expect(list.OutputToString()).To(Equal(secretName))

	})

	It("podman secret ls with filters", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).ToNot(HaveOccurred())

		secret1 := "Secret1"
		secret2 := "Secret2"

		session := podmanTest.Podman([]string{"secret", "ls", "-n"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal(""))

		session = podmanTest.Podman([]string{"secret", "ls", "--noheading"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(Equal(""))

		session = podmanTest.Podman([]string{"secret", "create", secret1, secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID1 := session.OutputToString()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"secret", "create", secret2, secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID2 := session.OutputToString()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"secret", "create", "Secret3", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		list := podmanTest.Podman([]string{"secret", "ls", "--filter", fmt.Sprintf("name=%s", secret1)})
		list.WaitWithDefaultTimeout()
		Expect(list).Should(Exit(0))
		Expect(list.OutputToStringArray()).To(HaveLen(2))
		Expect(list.OutputToStringArray()[1]).To(ContainSubstring(secret1))

		list = podmanTest.Podman([]string{"secret", "ls", "--filter", fmt.Sprintf("name=%s", secret2)})
		list.WaitWithDefaultTimeout()
		Expect(list).Should(Exit(0))
		Expect(list.OutputToStringArray()).To(HaveLen(2))
		Expect(list.OutputToStringArray()[1]).To(ContainSubstring(secret2))

		list = podmanTest.Podman([]string{"secret", "ls", "--filter", fmt.Sprintf("id=%s", secrID1)})
		list.WaitWithDefaultTimeout()
		Expect(list).Should(Exit(0))
		Expect(list.OutputToStringArray()).To(HaveLen(2))
		Expect(list.OutputToStringArray()[1]).To(ContainSubstring(secrID1))

		list = podmanTest.Podman([]string{"secret", "ls", "--filter", fmt.Sprintf("id=%s", secrID2)})
		list.WaitWithDefaultTimeout()
		Expect(list).Should(Exit(0))
		Expect(list.OutputToStringArray()).To(HaveLen(2))
		Expect(list.OutputToStringArray()[1]).To(ContainSubstring(secrID2))

		list = podmanTest.Podman([]string{"secret", "ls", "--filter", fmt.Sprintf("name=%s,name=%s", secret1, secret2)})
		list.WaitWithDefaultTimeout()
		Expect(list).Should(Exit(0))
		Expect(list.OutputToStringArray()).To(HaveLen(3))
		Expect(list.OutputToString()).To(ContainSubstring(secret1))
		Expect(list.OutputToString()).To(ContainSubstring(secret2))
	})

	It("podman secret ls with Go template", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		list := podmanTest.Podman([]string{"secret", "ls", "--format", "table {{.Name}}"})
		list.WaitWithDefaultTimeout()

		Expect(list).Should(Exit(0))
		Expect(list.OutputToStringArray()).To(HaveLen(2), list.OutputToString())
	})

	It("podman secret rm", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID := session.OutputToString()
		Expect(session).Should(Exit(0))

		removed := podmanTest.Podman([]string{"secret", "rm", "a"})
		removed.WaitWithDefaultTimeout()
		Expect(removed).Should(Exit(0))
		Expect(removed.OutputToString()).To(Equal(secrID))

		session = podmanTest.Podman([]string{"secret", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman secret rm --all", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"secret", "create", "b", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		removed := podmanTest.Podman([]string{"secret", "rm", "-a"})
		removed.WaitWithDefaultTimeout()
		Expect(removed).Should(Exit(0))

		session = podmanTest.Podman([]string{"secret", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman secret creates from environment variable", func() {
		// no env variable set, should fail
		session := podmanTest.Podman([]string{"secret", "create", "--env", "a", "MYENVVAR"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		os.Setenv("MYENVVAR", "somedata")
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}

		session = podmanTest.Podman([]string{"secret", "create", "--env", "a", "MYENVVAR"})
		session.WaitWithDefaultTimeout()
		secrID := session.OutputToString()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"secret", "inspect", "--format", "{{.ID}}", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(Equal(secrID))
	})

	It("podman secret with labels", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "--label", "foo=bar", "a", secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID := session.OutputToString()
		Expect(session).Should(Exit(0))

		inspect := podmanTest.Podman([]string{"secret", "inspect", "--format", "{{.Spec.Labels}}", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring("foo:bar"))

		session = podmanTest.Podman([]string{"secret", "create", "--label", "foo=bar", "--label", "a:b", "b", secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID = session.OutputToString()
		Expect(session).Should(Exit(0))

		inspect = podmanTest.Podman([]string{"secret", "inspect", "--format", "{{.Spec.Labels}}", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(ContainSubstring("foo:bar"))
		Expect(inspect.OutputToString()).To(ContainSubstring("a:b"))

		session = podmanTest.Podman([]string{"secret", "create", "c", secretFilePath})
		session.WaitWithDefaultTimeout()
		secrID = session.OutputToString()
		Expect(session).Should(Exit(0))

		inspect = podmanTest.Podman([]string{"secret", "inspect", "--format", "{{.Spec.Labels}}", secrID})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(Exit(0))
		Expect(inspect.OutputToString()).To(Equal("map[]"))

	})

	It("podman secret exists should return true if secret exists", func() {
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mysecret"), 0755)
		Expect(err).ToNot(HaveOccurred())

		secretName := "does_exist"

		session := podmanTest.Podman([]string{"secret", "create", secretName, secretFilePath})
		session.WaitWithDefaultTimeout()
		secretID := session.OutputToString()
		Expect(session).Should(Exit(0))

		exists := podmanTest.Podman([]string{"secret", "exists", secretName})
		exists.WaitWithDefaultTimeout()
		Expect(exists).Should(Exit(0))

		exists = podmanTest.Podman([]string{"secret", "exists", secretID})
		exists.WaitWithDefaultTimeout()
		Expect(exists).Should(Exit(0))
	})

	It("podman secret exists should return false if secret does not exist", func() {
		secretName := "does_not_exist"

		exists := podmanTest.Podman([]string{"secret", "exists", secretName})
		exists.WaitWithDefaultTimeout()
		Expect(exists).Should(Exit(1))
	})
})
