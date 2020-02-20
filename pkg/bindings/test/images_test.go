package test_bindings

import (
	"context"
	"net/http"
	"time"

	"github.com/containers/libpod/pkg/bindings"
	"github.com/containers/libpod/pkg/bindings/containers"
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
		bt        *bindingTest
		s         *gexec.Session
		connText  context.Context
		err       error
		falseFlag bool = false
		trueFlag  bool = true
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
		bt.RestoreImagesFromCache()
		s = bt.startAPIService()
		time.Sleep(1 * time.Second)
		connText, err = bindings.NewConnection(context.Background(), bt.sock)
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
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Inspect by short name
		data, err := images.GetImage(connText, alpine.shortName, nil)
		Expect(err).To(BeNil())

		// Inspect with full ID
		_, err = images.GetImage(connText, data.ID, nil)
		Expect(err).To(BeNil())

		// Inspect with partial ID
		_, err = images.GetImage(connText, data.ID[0:12], nil)
		Expect(err).To(BeNil())

		// Inspect by long name
		_, err = images.GetImage(connText, alpine.name, nil)
		Expect(err).To(BeNil())
	})

	// Test to validate the remove image api
	It("remove image", func() {
		// Remove invalid image should be a 404
		_, err = images.Remove(connText, "foobar5000", &falseFlag)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Remove an image by name, validate image is removed and error is nil
		inspectData, err := images.GetImage(connText, busybox.shortName, nil)
		Expect(err).To(BeNil())
		response, err := images.Remove(connText, busybox.shortName, nil)
		Expect(err).To(BeNil())
		Expect(inspectData.ID).To(Equal(response[0]["Deleted"]))
		inspectData, err = images.GetImage(connText, busybox.shortName, nil)
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Start a container with alpine image
		var top string = "top"
		bt.RunTopContainer(&top, &falseFlag, nil)
		// we should now have a container called "top" running
		containerResponse, err := containers.Inspect(connText, "top", &falseFlag)
		Expect(err).To(BeNil())
		Expect(containerResponse.Name).To(Equal("top"))

		// try to remove the image "alpine". This should fail since we are not force
		// deleting hence image cannot be deleted until the container is deleted.
		response, err = images.Remove(connText, alpine.shortName, &falseFlag)
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))

		// Removing the image "alpine" where force = true
		response, err = images.Remove(connText, alpine.shortName, &trueFlag)
		Expect(err).To(BeNil())

		// Checking if both the images are gone as well as the container is deleted
		inspectData, err = images.GetImage(connText, busybox.shortName, nil)
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		inspectData, err = images.GetImage(connText, alpine.shortName, nil)
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		_, err = containers.Inspect(connText, "top", &falseFlag)
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))
	})

	// Tests to validate the image tag command.
	It("tag image", func() {
		// Validates if invalid image name is given a bad response is encountered.
		err = images.Tag(connText, "dummy", "demo", alpine.shortName)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Validates if the image is tagged sucessfully.
		err = images.Tag(connText, alpine.shortName, "demo", alpine.shortName)
		Expect(err).To(BeNil())

		//Validates if name updates when the image is retagged.
		_, err := images.GetImage(connText, "alpine:demo", nil)
		Expect(err).To(BeNil())

	})

	// Test to validate the List images command.
	It("List image", func() {
		// Array to hold the list of images returned
		imageSummary, err := images.List(connText, nil, nil)
		// There Should be no errors in the response.
		Expect(err).To(BeNil())
		// Since in the begin context two images are created the
		// list context should have only 2 images
		Expect(len(imageSummary)).To(Equal(2))

		// Adding one more image. There Should be no errors in the response.
		// And the count should be three now.
		bt.Pull("busybox:glibc")
		imageSummary, err = images.List(connText, nil, nil)
		Expect(err).To(BeNil())
		Expect(len(imageSummary)).To(Equal(3))

		//Validate the image names.
		var names []string
		for _, i := range imageSummary {
			names = append(names, i.RepoTags...)
		}
		Expect(StringInSlice(alpine.name, names)).To(BeTrue())
		Expect(StringInSlice(busybox.name, names)).To(BeTrue())

		// List  images with a filter
		filters := make(map[string][]string)
		filters["reference"] = []string{alpine.name}
		filteredImages, err := images.List(connText, &falseFlag, filters)
		Expect(err).To(BeNil())
		Expect(len(filteredImages)).To(BeNumerically("==", 1))

		// List  images with a bad filter
		filters["name"] = []string{alpine.name}
		_, err = images.List(connText, &falseFlag, filters)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

})
