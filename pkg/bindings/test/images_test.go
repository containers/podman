package test_bindings

import (
	"context"
	"fmt"
	"time"

	"github.com/containers/libpod/pkg/bindings"
	"github.com/containers/libpod/pkg/bindings/images"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman images", func() {
	var (
		//tempdir    string
		//err        error
		//podmanTest *PodmanTestIntegration
		bt       *bindingTest
		s        *gexec.Session
		connText context.Context
		err      error
		false    bool
		//true bool = true
	)

	BeforeEach(func() {
		//tempdir, err = CreateTempDirInTempDir()
		//if err != nil {
		//	os.Exit(1)
		//}
		//podmanTest = PodmanTestCreate(tempdir)
		//podmanTest.Setup()
		//podmanTest.SeedImages()
		bt = newBindingTest()
		p := bt.runPodman([]string{"pull", alpine})
		p.Wait(45)
		s = bt.startAPIService()
		time.Sleep(1 * time.Second)
		connText, err = bindings.NewConnection(bt.sock)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		//podmanTest.Cleanup()
		//f := CurrentGinkgoTestDescription()
		//processTestResult(f)
		s.Kill()
		bt.cleanup()
	})
	It("inspect image", func() {
		// Inspect invalid image be 404
		_, err = images.GetImage(connText, "foobar5000", nil)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", 404))

		// Inspect by short name
		data, err := images.GetImage(connText, "alpine", nil)
		Expect(err).To(BeNil())

		// Inspect with full ID
		_, err = images.GetImage(connText, data.ID, nil)
		Expect(err).To(BeNil())

		// Inspect with partial ID
		_, err = images.GetImage(connText, data.ID[0:12], nil)
		Expect(err).To(BeNil())

		//Inspect by long name should work, it doesnt (yet) i think it needs to be html escaped
		_, err = images.GetImage(connText, alpine, nil)
		Expect(err).To(BeNil())
	})
	It("remove image", func() {
		// Remove invalid image should be a 404
		_, err = images.Remove(connText, "foobar5000", &false)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", 404))

		_, err := images.GetImage(connText, "alpine", nil)
		Expect(err).To(BeNil())

		response, err := images.Remove(connText, "alpine", &false)
		Expect(err).To(BeNil())
		fmt.Println(response)
		//	to be continued

	})

	//Tests to validate the image tag command.
	It("tag image", func() {
		// Validates if invalid image name is given a bad response is encountered.
		err = images.Tag(connText, "dummy", "demo", "alpine")
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", 404))

		// Validates if the image is tagged sucessfully.
		err = images.Tag(connText, "alpine", "demo", "alpine")
		Expect(err).To(BeNil())

		//Validates if name updates when the image is retagged.
		_, err := images.GetImage(connText, "alpine:demo", nil)
		Expect(err).To(BeNil())

	})

	//Test to validate the List images command.
	It("List image", func() {
		//Array to hold the list of images returned
		imageSummary, err := images.List(connText, nil, nil)
		//There Should be no errors in the response.
		Expect(err).To(BeNil())
		//Since in the begin context only one image is created the list context should have only one image
		Expect(len(imageSummary)).To(Equal(1))

		//To be written create a new image and check list count again
		//imageSummary, err = images.List(connText, nil, nil)

		//Since in the begin context only one image adding one more image should
		///Expect(len(imageSummary)).To(Equal(2)

	})

})
