package bindings_test

import (
	"net/http"
	"time"

	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/bindings/images"
	"github.com/containers/podman/v4/pkg/bindings/manifests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("podman manifest", func() {
	var (
		bt *bindingTest
		s  *gexec.Session
	)

	BeforeEach(func() {
		bt = newBindingTest()
		bt.RestoreImagesFromCache()
		s = bt.startAPIService()
		time.Sleep(1 * time.Second)
		err := bt.NewConnection()
		Expect(err).ToNot(HaveOccurred())

	})

	AfterEach(func() {
		s.Kill()
		bt.cleanup()
	})

	It("create", func() {
		// create manifest list without images
		id, err := manifests.Create(bt.conn, "quay.io/libpod/foobar:latest", []string{}, nil)
		Expect(err).ToNot(HaveOccurred(), err)
		list, err := manifests.Inspect(bt.conn, id, nil)
		Expect(err).ToNot(HaveOccurred())

		Expect(len(list.Manifests)).To(BeZero())

		// creating a duplicate should fail as a 500
		_, err = manifests.Create(bt.conn, "quay.io/libpod/foobar:latest", nil, nil)
		Expect(err).To(HaveOccurred())

		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))

		_, errs := images.Remove(bt.conn, []string{id}, nil)
		Expect(len(errs)).To(BeZero())

		// create manifest list with images
		id, err = manifests.Create(bt.conn, "quay.io/libpod/foobar:latest", []string{alpine.name}, nil)
		Expect(err).ToNot(HaveOccurred())

		list, err = manifests.Inspect(bt.conn, id, nil)
		Expect(err).ToNot(HaveOccurred())

		Expect(len(list.Manifests)).To(BeNumerically("==", 1))
	})

	It("inspect", func() {
		_, err := manifests.Inspect(bt.conn, "larry", nil)
		Expect(err).To(HaveOccurred())

		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))
	})

	It("add", func() {
		// add to bogus should 404
		_, err := manifests.Add(bt.conn, "foobar", nil)
		Expect(err).To(HaveOccurred())

		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound), err.Error())

		id, err := manifests.Create(bt.conn, "quay.io/libpod/foobar:latest", []string{}, nil)
		Expect(err).ToNot(HaveOccurred())

		options := new(manifests.AddOptions).WithImages([]string{alpine.name})
		_, err = manifests.Add(bt.conn, id, options)
		Expect(err).ToNot(HaveOccurred())

		list, err := manifests.Inspect(bt.conn, id, nil)
		Expect(err).ToNot(HaveOccurred())

		Expect(len(list.Manifests)).To(BeNumerically("==", 1))

		// add bogus name to existing list should fail
		options.WithImages([]string{"larry"})
		_, err = manifests.Add(bt.conn, id, options)
		Expect(err).To(HaveOccurred())

		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("remove digest", func() {
		// removal on bogus manifest list should be 404
		_, err := manifests.Remove(bt.conn, "larry", "1234", nil)
		Expect(err).To(HaveOccurred())

		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		id, err := manifests.Create(bt.conn, "quay.io/libpod/foobar:latest", []string{alpine.name}, nil)
		Expect(err).ToNot(HaveOccurred())

		data, err := manifests.Inspect(bt.conn, id, nil)
		Expect(err).ToNot(HaveOccurred())

		Expect(len(data.Manifests)).To(BeNumerically("==", 1))

		// removal on a good manifest list with a bad digest should be 400
		_, err = manifests.Remove(bt.conn, id, "!234", nil)
		Expect(err).To(HaveOccurred())

		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusBadRequest))

		digest := data.Manifests[0].Digest.String()
		_, err = manifests.Remove(bt.conn, id, digest, nil)
		Expect(err).ToNot(HaveOccurred())

		// removal on good manifest with good digest should work
		data, err = manifests.Inspect(bt.conn, id, nil)
		Expect(err).ToNot(HaveOccurred())

		Expect(data.Manifests).Should(BeEmpty())
	})

	It("annotate", func() {
		id, err := manifests.Create(bt.conn, "quay.io/libpod/foobar:latest", []string{}, nil)
		Expect(err).ToNot(HaveOccurred())

		opts := manifests.AddOptions{Images: []string{"quay.io/libpod/alpine:latest"}}

		_, err = manifests.Add(bt.conn, id, &opts)
		Expect(err).ToNot(HaveOccurred())

		data, err := manifests.Inspect(bt.conn, id, nil)
		Expect(err).ToNot(HaveOccurred())

		Expect(len(data.Manifests)).To(BeNumerically("==", 1))

		digest := data.Manifests[0].Digest.String()
		annoOpts := new(manifests.ModifyOptions).WithOS("foo")
		_, err = manifests.Annotate(bt.conn, id, []string{digest}, annoOpts)
		Expect(err).ToNot(HaveOccurred())

		list, err := manifests.Inspect(bt.conn, id, nil)
		Expect(err).ToNot(HaveOccurred())

		Expect(len(list.Manifests)).To(BeNumerically("==", 1))
		Expect(list.Manifests[0].Platform.OS).To(Equal("foo"))
	})

	It("push manifest", func() {
		Skip("TODO: implement test for manifest push to registry")
	})
})
