// +build !remoteclient

package integration

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/containers/libpod/test/utils"
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
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
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
		Expect(outArray[0]).Should(ContainSubstring("accept"))
		Expect(outArray[1]).Should(ContainSubstring("reject"))
		Expect(outArray[2]).Should(ContainSubstring("signed"))
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
})
