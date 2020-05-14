package test_bindings

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/containers/libpod/pkg/bindings"
	"github.com/containers/libpod/pkg/bindings/containers"
	"github.com/containers/libpod/pkg/bindings/images"
	"github.com/containers/libpod/pkg/domain/entities"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman images", func() {
	var (
		// tempdir    string
		// err        error
		// podmanTest *PodmanTestIntegration
		bt  *bindingTest
		s   *gexec.Session
		err error
	)

	BeforeEach(func() {
		// tempdir, err = CreateTempDirInTempDir()
		// if err != nil {
		//	os.Exit(1)
		// }
		// podmanTest = PodmanTestCreate(tempdir)
		// podmanTest.Setup()
		// podmanTest.SeedImages()
		bt = newBindingTest()
		bt.RestoreImagesFromCache()
		s = bt.startAPIService()
		time.Sleep(1 * time.Second)
		err := bt.NewConnection()
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		// podmanTest.Cleanup()
		// f := CurrentGinkgoTestDescription()
		// processTestResult(f)
		s.Kill()
		bt.cleanup()
	})

	It("inspect image", func() {
		// Inspect invalid image be 404
		_, err = images.GetImage(bt.conn, "foobar5000", nil)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Inspect by short name
		data, err := images.GetImage(bt.conn, alpine.shortName, nil)
		Expect(err).To(BeNil())

		// Inspect with full ID
		_, err = images.GetImage(bt.conn, data.ID, nil)
		Expect(err).To(BeNil())

		// Inspect with partial ID
		_, err = images.GetImage(bt.conn, data.ID[0:12], nil)
		Expect(err).To(BeNil())

		// Inspect by long name
		_, err = images.GetImage(bt.conn, alpine.name, nil)
		Expect(err).To(BeNil())
		// TODO it looks like the images API alwaays returns size regardless
		// of bool or not. What should we do ?
		// Expect(data.Size).To(BeZero())

		// Enabling the size parameter should result in size being populated
		data, err = images.GetImage(bt.conn, alpine.name, bindings.PTrue)
		Expect(err).To(BeNil())
		Expect(data.Size).To(BeNumerically(">", 0))
	})

	// Test to validate the remove image api
	It("remove image", func() {
		// Remove invalid image should be a 404
		response, err := images.Remove(bt.conn, "foobar5000", false)
		Expect(err).ToNot(BeNil())
		Expect(response).To(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Remove an image by name, validate image is removed and error is nil
		inspectData, err := images.GetImage(bt.conn, busybox.shortName, nil)
		Expect(err).To(BeNil())
		response, err = images.Remove(bt.conn, busybox.shortName, false)
		Expect(err).To(BeNil())
		code, _ = bindings.CheckResponseCode(err)

		Expect(inspectData.ID).To(Equal(response.Deleted[0]))
		inspectData, err = images.GetImage(bt.conn, busybox.shortName, nil)
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Start a container with alpine image
		var top string = "top"
		_, err = bt.RunTopContainer(&top, bindings.PFalse, nil)
		Expect(err).To(BeNil())
		// we should now have a container called "top" running
		containerResponse, err := containers.Inspect(bt.conn, "top", nil)
		Expect(err).To(BeNil())
		Expect(containerResponse.Name).To(Equal("top"))

		// try to remove the image "alpine". This should fail since we are not force
		// deleting hence image cannot be deleted until the container is deleted.
		response, err = images.Remove(bt.conn, alpine.shortName, false)
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusConflict))

		// Removing the image "alpine" where force = true
		response, err = images.Remove(bt.conn, alpine.shortName, true)
		Expect(err).To(BeNil())
		// To be extra sure, check if the previously created container
		// is gone as well.
		_, err = containers.Inspect(bt.conn, "top", bindings.PFalse)
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Now make sure both images are gone.
		inspectData, err = images.GetImage(bt.conn, busybox.shortName, nil)
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		inspectData, err = images.GetImage(bt.conn, alpine.shortName, nil)
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))
	})

	// Tests to validate the image tag command.
	It("tag image", func() {
		// Validates if invalid image name is given a bad response is encountered.
		err = images.Tag(bt.conn, "dummy", "demo", alpine.shortName)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Validates if the image is tagged successfully.
		err = images.Tag(bt.conn, alpine.shortName, "demo", alpine.shortName)
		Expect(err).To(BeNil())

		// Validates if name updates when the image is retagged.
		_, err := images.GetImage(bt.conn, "alpine:demo", nil)
		Expect(err).To(BeNil())

	})

	// Test to validate the List images command.
	It("List image", func() {
		// Array to hold the list of images returned
		imageSummary, err := images.List(bt.conn, nil, nil)
		// There Should be no errors in the response.
		Expect(err).To(BeNil())
		// Since in the begin context two images are created the
		// list context should have only 2 images
		Expect(len(imageSummary)).To(Equal(2))

		// Adding one more image. There Should be no errors in the response.
		// And the count should be three now.
		bt.Pull("busybox:glibc")
		imageSummary, err = images.List(bt.conn, nil, nil)
		Expect(err).To(BeNil())
		Expect(len(imageSummary)).To(Equal(3))

		// Validate the image names.
		var names []string
		for _, i := range imageSummary {
			names = append(names, i.RepoTags...)
		}
		Expect(StringInSlice(alpine.name, names)).To(BeTrue())
		Expect(StringInSlice(busybox.name, names)).To(BeTrue())

		// List  images with a filter
		filters := make(map[string][]string)
		filters["reference"] = []string{alpine.name}
		filteredImages, err := images.List(bt.conn, bindings.PFalse, filters)
		Expect(err).To(BeNil())
		Expect(len(filteredImages)).To(BeNumerically("==", 1))

		// List  images with a bad filter
		filters["name"] = []string{alpine.name}
		_, err = images.List(bt.conn, bindings.PFalse, filters)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("Image Exists", func() {
		// exists on bogus image should be false, with no error
		exists, err := images.Exists(bt.conn, "foobar")
		Expect(err).To(BeNil())
		Expect(exists).To(BeFalse())

		// exists with shortname should be true
		exists, err = images.Exists(bt.conn, alpine.shortName)
		Expect(err).To(BeNil())
		Expect(exists).To(BeTrue())

		// exists with fqname should be true
		exists, err = images.Exists(bt.conn, alpine.name)
		Expect(err).To(BeNil())
		Expect(exists).To(BeTrue())
	})

	It("Load|Import Image", func() {
		// load an image
		_, err := images.Remove(bt.conn, alpine.name, false)
		Expect(err).To(BeNil())
		exists, err := images.Exists(bt.conn, alpine.name)
		Expect(err).To(BeNil())
		Expect(exists).To(BeFalse())
		f, err := os.Open(filepath.Join(ImageCacheDir, alpine.tarballName))
		defer f.Close()
		Expect(err).To(BeNil())
		names, err := images.Load(bt.conn, f, nil)
		Expect(err).To(BeNil())
		Expect(names.Names[0]).To(Equal(alpine.name))
		exists, err = images.Exists(bt.conn, alpine.name)
		Expect(err).To(BeNil())
		Expect(exists).To(BeTrue())

		// load with a repo name
		f, err = os.Open(filepath.Join(ImageCacheDir, alpine.tarballName))
		Expect(err).To(BeNil())
		_, err = images.Remove(bt.conn, alpine.name, false)
		Expect(err).To(BeNil())
		exists, err = images.Exists(bt.conn, alpine.name)
		Expect(err).To(BeNil())
		Expect(exists).To(BeFalse())
		newName := "quay.io/newname:fizzle"
		names, err = images.Load(bt.conn, f, &newName)
		Expect(err).To(BeNil())
		Expect(names.Names[0]).To(Equal(alpine.name))
		exists, err = images.Exists(bt.conn, newName)
		Expect(err).To(BeNil())
		Expect(exists).To(BeTrue())

		// load with a bad repo name should trigger a 500
		f, err = os.Open(filepath.Join(ImageCacheDir, alpine.tarballName))
		Expect(err).To(BeNil())
		_, err = images.Remove(bt.conn, alpine.name, false)
		Expect(err).To(BeNil())
		exists, err = images.Exists(bt.conn, alpine.name)
		Expect(err).To(BeNil())
		Expect(exists).To(BeFalse())
		badName := "quay.io/newName:fizzle"
		_, err = images.Load(bt.conn, f, &badName)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("Export Image", func() {
		// Export an image
		exportPath := filepath.Join(bt.tempDirPath, alpine.tarballName)
		w, err := os.Create(filepath.Join(bt.tempDirPath, alpine.tarballName))
		defer w.Close()
		Expect(err).To(BeNil())
		err = images.Export(bt.conn, alpine.name, w, nil, nil)
		Expect(err).To(BeNil())
		_, err = os.Stat(exportPath)
		Expect(err).To(BeNil())

		// TODO how do we verify that a format change worked?
	})

	It("Import Image", func() {
		// load an image
		_, err = images.Remove(bt.conn, alpine.name, false)
		Expect(err).To(BeNil())
		exists, err := images.Exists(bt.conn, alpine.name)
		Expect(err).To(BeNil())
		Expect(exists).To(BeFalse())
		f, err := os.Open(filepath.Join(ImageCacheDir, alpine.tarballName))
		defer f.Close()
		Expect(err).To(BeNil())
		changes := []string{"CMD /bin/foobar"}
		testMessage := "test_import"
		_, err = images.Import(bt.conn, changes, &testMessage, &alpine.name, nil, f)
		Expect(err).To(BeNil())
		exists, err = images.Exists(bt.conn, alpine.name)
		Expect(err).To(BeNil())
		Expect(exists).To(BeTrue())
		data, err := images.GetImage(bt.conn, alpine.name, nil)
		Expect(err).To(BeNil())
		Expect(data.Comment).To(Equal(testMessage))

	})

	It("History Image", func() {
		// a bogus name should return a 404
		_, err := images.History(bt.conn, "foobar")
		Expect(err).To(Not(BeNil()))
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		var foundID bool
		data, err := images.GetImage(bt.conn, alpine.name, nil)
		Expect(err).To(BeNil())
		history, err := images.History(bt.conn, alpine.name)
		Expect(err).To(BeNil())
		for _, i := range history {
			if i.ID == data.ID {
				foundID = true
				break
			}
		}
		Expect(foundID).To(BeTrue())
	})

	It("Search for an image", func() {
		reports, err := images.Search(bt.conn, "alpine", entities.ImageSearchOptions{})
		Expect(err).To(BeNil())
		Expect(len(reports)).To(BeNumerically(">", 1))
		var foundAlpine bool
		for _, i := range reports {
			if i.Name == "docker.io/library/alpine" {
				foundAlpine = true
				break
			}
		}
		Expect(foundAlpine).To(BeTrue())

		// Search for alpine with a limit of 10
		reports, err = images.Search(bt.conn, "docker.io/alpine", entities.ImageSearchOptions{Limit: 10})
		Expect(err).To(BeNil())
		Expect(len(reports)).To(BeNumerically("<=", 10))

		// Search for alpine with stars greater than 100
		reports, err = images.Search(bt.conn, "docker.io/alpine", entities.ImageSearchOptions{Filters: []string{"stars=100"}})
		Expect(err).To(BeNil())
		for _, i := range reports {
			Expect(i.Stars).To(BeNumerically(">=", 100))
		}

		//	Search with a fqdn
		reports, err = images.Search(bt.conn, "quay.io/libpod/alpine_nginx", entities.ImageSearchOptions{})
		Expect(len(reports)).To(BeNumerically(">=", 1))
	})

	It("Prune images", func() {
		trueBoxed := true
		results, err := images.Prune(bt.conn, &trueBoxed, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(results)).To(BeNumerically(">", 0))
		Expect(results).To(ContainElement("docker.io/library/alpine:latest"))
	})

	// TODO: we really need to extent to pull tests once we have a more sophisticated CI.
	It("Image Pull", func() {
		rawImage := "docker.io/library/busybox:latest"

		pulledImages, err := images.Pull(bt.conn, rawImage, entities.ImagePullOptions{})
		Expect(err).To(BeNil())
		Expect(len(pulledImages)).To(Equal(1))

		exists, err := images.Exists(bt.conn, rawImage)
		Expect(err).To(BeNil())
		Expect(exists).To(BeTrue())

		// Make sure the normalization AND the full-transport reference works.
		_, err = images.Pull(bt.conn, "docker://"+rawImage, entities.ImagePullOptions{})
		Expect(err).To(BeNil())

		// The v2 endpoint only supports the docker transport.  Let's see if that's really true.
		_, err = images.Pull(bt.conn, "bogus-transport:bogus.com/image:reference", entities.ImagePullOptions{})
		Expect(err).To(Not(BeNil()))
	})
})
