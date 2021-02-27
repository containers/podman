package integration

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman trust", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		SkipIfRemote("podman-remote does not support image trust")
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

	It("podman image trust show", func() {
		path, err := os.Getwd()
		if err != nil {
			os.Exit(1)
		}
		session := podmanTest.Podman([]string{"image", "trust", "show", "--registrypath", filepath.Dir(path), "--policypath", filepath.Join(filepath.Dir(path), "policy.json")})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		outArray := session.OutputToStringArray()
		Expect(len(outArray)).To(Equal(3))

		// image order is not guaranteed. All we can do is check that
		// these strings appear in output, we can't cross-check them.
		Expect(session.OutputToString()).To(ContainSubstring("accept"))
		Expect(session.OutputToString()).To(ContainSubstring("reject"))
		Expect(session.OutputToString()).To(ContainSubstring("signed"))
	})

	It("podman image trust set", func() {
		path, err := os.Getwd()
		if err != nil {
			os.Exit(1)
		}
		session := podmanTest.Podman([]string{"image", "trust", "set", "--policypath", filepath.Join(filepath.Dir(path), "trust_set_test.json"), "-t", "accept", "default"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		var teststruct map[string][]map[string]string
		policyContent, err := ioutil.ReadFile(filepath.Join(filepath.Dir(path), "trust_set_test.json"))
		if err != nil {
			os.Exit(1)
		}
		err = json.Unmarshal(policyContent, &teststruct)
		if err != nil {
			os.Exit(1)
		}
		Expect(teststruct["default"][0]["type"]).To(Equal("insecureAcceptAnything"))
	})

	It("podman image trust show --json", func() {
		session := podmanTest.Podman([]string{"image", "trust", "show", "--json"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.IsJSONOutputValid()).To(BeTrue())
		var teststruct []map[string]string
		json.Unmarshal(session.Out.Contents(), &teststruct)
		Expect(teststruct[0]["name"]).To(Equal("* (default)"))
		Expect(teststruct[0]["repo_name"]).To(Equal("default"))
		Expect(teststruct[0]["type"]).To(Equal("accept"))
		Expect(teststruct[1]["type"]).To(Equal("insecureAcceptAnything"))
	})

	It("podman image trust show --raw", func() {
		session := podmanTest.Podman([]string{"image", "trust", "show", "--raw"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.IsJSONOutputValid()).To(BeTrue())
		Expect(session.OutputToString()).To(ContainSubstring("default"))
		Expect(session.OutputToString()).To(ContainSubstring("insecureAcceptAnything"))
	})
})
