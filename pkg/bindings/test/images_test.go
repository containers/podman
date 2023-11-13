package test_bindings

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/podman/v3/pkg/bindings/containers"
	"github.com/containers/podman/v3/pkg/bindings/images"
	dreports "github.com/containers/podman/v3/pkg/domain/entities/reports"
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
		// TODO it looks like the images API always returns size regardless
		// of bool or not. What should we do ?
		// Expect(data.Size).To(BeZero())

		options := new(images.GetOptions).WithSize(true)
		// Enabling the size parameter should result in size being populated
		data, err = images.GetImage(bt.conn, alpine.name, options)
		Expect(err).To(BeNil())
		Expect(data.Size).To(BeNumerically(">", 0))
	})

	// Test to validate the remove image api
	It("remove image", func() {
		// Remove invalid image should be a 404
		response, errs := images.Remove(bt.conn, []string{"foobar5000"}, nil)
		Expect(len(errs)).To(BeNumerically(">", 0))
		code, _ := bindings.CheckResponseCode(errs[0])

		// Remove an image by name, validate image is removed and error is nil
		inspectData, err := images.GetImage(bt.conn, busybox.shortName, nil)
		Expect(err).To(BeNil())
		response, errs = images.Remove(bt.conn, []string{busybox.shortName}, nil)
		Expect(len(errs)).To(BeZero())

		Expect(inspectData.ID).To(Equal(response.Deleted[0]))
		inspectData, err = images.GetImage(bt.conn, busybox.shortName, nil)
		code, _ = bindings.CheckResponseCode(err)

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
		response, errs = images.Remove(bt.conn, []string{alpine.shortName}, nil)
		code, _ = bindings.CheckResponseCode(errs[0])

		// Removing the image "alpine" where force = true
		options := new(images.RemoveOptions).WithForce(true)
		response, errs = images.Remove(bt.conn, []string{alpine.shortName}, options)
		Expect(len(errs)).To(BeZero())
		// To be extra sure, check if the previously created container
		// is gone as well.
		_, err = containers.Inspect(bt.conn, "top", nil)
		code, _ = bindings.CheckResponseCode(err)

		// Now make sure both images are gone.
		inspectData, err = images.GetImage(bt.conn, busybox.shortName, nil)
		code, _ = bindings.CheckResponseCode(err)

		inspectData, err = images.GetImage(bt.conn, alpine.shortName, nil)
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))
	})

	// Tests to validate the image tag command.
	It("tag image", func() {

		// Validates if invalid image name is given a bad response is encountered.
		err = images.Tag(bt.conn, "dummy", "demo", alpine.shortName, nil)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Validates if the image is tagged successfully.
		err = images.Tag(bt.conn, alpine.shortName, "demo", alpine.shortName, nil)
		Expect(err).To(BeNil())

		// Validates if name updates when the image is retagged.
		_, err := images.GetImage(bt.conn, "alpine:demo", nil)
		Expect(err).To(BeNil())

	})

	// Test to validate the List images command.
	It("List image", func() {
		// Array to hold the list of images returned
		imageSummary, err := images.List(bt.conn, nil)
		// There Should be no errors in the response.
		Expect(err).To(BeNil())
		// Since in the begin context two images are created the
		// list context should have only 2 images
		Expect(len(imageSummary)).To(Equal(2))

		// Adding one more image. There Should be no errors in the response.
		// And the count should be three now.
		bt.Pull("testimage:20200929")
		imageSummary, err = images.List(bt.conn, nil)
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
		options := new(images.ListOptions).WithFilters(filters).WithAll(false)
		filteredImages, err := images.List(bt.conn, options)
		Expect(err).To(BeNil())
		Expect(len(filteredImages)).To(BeNumerically("==", 1))

		// List  images with a bad filter
		filters["name"] = []string{alpine.name}
		options = new(images.ListOptions).WithFilters(filters)
		_, err = images.List(bt.conn, options)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("Image Exists", func() {
		// exists on bogus image should be false, with no error
		exists, err := images.Exists(bt.conn, "foobar", nil)
		Expect(err).To(BeNil())
		Expect(exists).To(BeFalse())

		// exists with shortname should be true
		exists, err = images.Exists(bt.conn, alpine.shortName, nil)
		Expect(err).To(BeNil())
		Expect(exists).To(BeTrue())

		// exists with fqname should be true
		exists, err = images.Exists(bt.conn, alpine.name, nil)
		Expect(err).To(BeNil())
		Expect(exists).To(BeTrue())
	})

	It("Load|Import Image", func() {
		// load an image
		_, errs := images.Remove(bt.conn, []string{alpine.name}, nil)
		Expect(len(errs)).To(BeZero())
		exists, err := images.Exists(bt.conn, alpine.name, nil)
		Expect(err).To(BeNil())
		Expect(exists).To(BeFalse())
		f, err := os.Open(filepath.Join(ImageCacheDir, alpine.tarballName))
		defer f.Close()
		Expect(err).To(BeNil())
		names, err := images.Load(bt.conn, f)
		Expect(err).To(BeNil())
		Expect(names.Names[0]).To(Equal(alpine.name))
		exists, err = images.Exists(bt.conn, alpine.name, nil)
		Expect(err).To(BeNil())
		Expect(exists).To(BeTrue())

		// load with a repo name
		f, err = os.Open(filepath.Join(ImageCacheDir, alpine.tarballName))
		Expect(err).To(BeNil())
		_, errs = images.Remove(bt.conn, []string{alpine.name}, nil)
		Expect(len(errs)).To(BeZero())
		exists, err = images.Exists(bt.conn, alpine.name, nil)
		Expect(err).To(BeNil())
		Expect(exists).To(BeFalse())
		names, err = images.Load(bt.conn, f)
		Expect(err).To(BeNil())
		Expect(names.Names[0]).To(Equal(alpine.name))

		// load with a bad repo name should trigger a 500
		f, err = os.Open(filepath.Join(ImageCacheDir, alpine.tarballName))
		Expect(err).To(BeNil())
		_, errs = images.Remove(bt.conn, []string{alpine.name}, nil)
		Expect(len(errs)).To(BeZero())
		exists, err = images.Exists(bt.conn, alpine.name, nil)
		Expect(err).To(BeNil())
		Expect(exists).To(BeFalse())
	})

	It("Export Image", func() {
		// Export an image
		exportPath := filepath.Join(bt.tempDirPath, alpine.tarballName)
		w, err := os.Create(filepath.Join(bt.tempDirPath, alpine.tarballName))
		defer w.Close()
		Expect(err).To(BeNil())
		err = images.Export(bt.conn, []string{alpine.name}, w, nil)
		Expect(err).To(BeNil())
		_, err = os.Stat(exportPath)
		Expect(err).To(BeNil())

		// TODO how do we verify that a format change worked?
	})

	It("Import Image", func() {
		// load an image
		_, errs := images.Remove(bt.conn, []string{alpine.name}, nil)
		Expect(len(errs)).To(BeZero())
		exists, err := images.Exists(bt.conn, alpine.name, nil)
		Expect(err).To(BeNil())
		Expect(exists).To(BeFalse())
		f, err := os.Open(filepath.Join(ImageCacheDir, alpine.tarballName))
		defer f.Close()
		Expect(err).To(BeNil())
		changes := []string{"CMD /bin/foobar"}
		testMessage := "test_import"
		options := new(images.ImportOptions).WithMessage(testMessage).WithChanges(changes).WithReference(alpine.name)
		_, err = images.Import(bt.conn, f, options)
		Expect(err).To(BeNil())
		exists, err = images.Exists(bt.conn, alpine.name, nil)
		Expect(err).To(BeNil())
		Expect(exists).To(BeTrue())
		data, err := images.GetImage(bt.conn, alpine.name, nil)
		Expect(err).To(BeNil())
		Expect(data.Comment).To(Equal(testMessage))

	})

	It("History Image", func() {
		// a bogus name should return a 404
		_, err := images.History(bt.conn, "foobar", nil)
		Expect(err).To(Not(BeNil()))
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		var foundID bool
		data, err := images.GetImage(bt.conn, alpine.name, nil)
		Expect(err).To(BeNil())
		history, err := images.History(bt.conn, alpine.name, nil)
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
		reports, err := images.Search(bt.conn, "alpine", nil)
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
		options := new(images.SearchOptions).WithLimit(10)
		reports, err = images.Search(bt.conn, "docker.io/alpine", options)
		Expect(err).To(BeNil())
		Expect(len(reports)).To(BeNumerically("<=", 10))

		filters := make(map[string][]string)
		filters["stars"] = []string{"100"}
		// Search for alpine with stars greater than 100
		options = new(images.SearchOptions).WithFilters(filters)
		reports, err = images.Search(bt.conn, "docker.io/alpine", options)
		Expect(err).To(BeNil())
		for _, i := range reports {
			Expect(i.Stars).To(BeNumerically(">=", 100))
		}

		//	Search with a fqdn
		reports, err = images.Search(bt.conn, "quay.io/podman/stable", nil)
		Expect(len(reports)).To(BeNumerically(">=", 1))
	})

	It("Prune images", func() {
		options := new(images.PruneOptions).WithAll(true)
		results, err := images.Prune(bt.conn, options)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(results)).To(BeNumerically(">", 0))
		Expect(dreports.PruneReportsIds(results)).To(ContainElement("docker.io/library/alpine:latest"))
	})

	// TODO: we really need to extent to pull tests once we have a more sophisticated CI.
	It("Image Pull", func() {
		rawImage := "docker.io/library/busybox:latest"

		pulledImages, err := images.Pull(bt.conn, rawImage, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(pulledImages)).To(Equal(1))

		exists, err := images.Exists(bt.conn, rawImage, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(exists).To(BeTrue())

		// Make sure the normalization AND the full-transport reference works.
		_, err = images.Pull(bt.conn, "docker://"+rawImage, nil)
		Expect(err).NotTo(HaveOccurred())

		// The v2 endpoint only supports the docker transport.  Let's see if that's really true.
		_, err = images.Pull(bt.conn, "bogus-transport:bogus.com/image:reference", nil)
		Expect(err).To(HaveOccurred())
	})
})
