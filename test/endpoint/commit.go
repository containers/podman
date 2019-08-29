package endpoint

import (
	"encoding/json"
	"os"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman commit", func() {
	var (
		tempdir      string
		err          error
		endpointTest *EndpointTestIntegration
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		endpointTest = Setup(tempdir)
		endpointTest.StartVarlinkWithCache()
	})

	AfterEach(func() {
		endpointTest.Cleanup()

	})

	It("ensure commit with uppercase image name does not panic", func() {
		body := make(map[string]string)
		body["image_name"] = "FOO"
		body["format"] = "oci"
		body["name"] = "top"
		b, err := json.Marshal(body)
		Expect(err).To(BeNil())
		// run the container to be committed
		_ = endpointTest.startTopContainer("top")
		result := endpointTest.Varlink("Commit", string(b), false)
		// This indicates an error occured
		Expect(len(result.StdErrToString())).To(BeNumerically(">", 0))
	})

})
