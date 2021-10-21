package integration

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman trust", func() {
	var (
		tempdir string

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
		session := podmanTest.Podman([]string{"image", "trust", "show", "--registrypath", filepath.Join(INTEGRATION_ROOT, "test"), "--policypath", filepath.Join(INTEGRATION_ROOT, "test/policy.json")})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		outArray := session.OutputToStringArray()
		Expect(len(outArray)).To(Equal(3))

		// Repository order is not guaranteed. So, check that
		// all expected lines appear in output; we also check total number of lines, so that handles all of them.
		Expect(string(session.Out.Contents())).To(MatchRegexp(`(?m)^default\s+accept\s*$`))
		Expect(string(session.Out.Contents())).To(MatchRegexp(`(?m)^docker.io/library/hello-world\s+reject\s*$`))
		Expect(string(session.Out.Contents())).To(MatchRegexp(`(?m)^registry.access.redhat.com\s+signedBy\s+security@redhat.com, security@redhat.com\s+https://access.redhat.com/webassets/docker/content/sigstore\s*$`))
	})

	It("podman image trust set", func() {
		path, err := os.Getwd()
		if err != nil {
			os.Exit(1)
		}
		session := podmanTest.Podman([]string{"image", "trust", "set", "--policypath", filepath.Join(filepath.Dir(path), "trust_set_test.json"), "-t", "accept", "default"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
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
		Expect(session).Should(Exit(0))
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
		Expect(session).Should(Exit(0))
		Expect(session.IsJSONOutputValid()).To(BeTrue())
		Expect(session.OutputToString()).To(ContainSubstring("default"))
		Expect(session.OutputToString()).To(ContainSubstring("insecureAcceptAnything"))
	})
})
