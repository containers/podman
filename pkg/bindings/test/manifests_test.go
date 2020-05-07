package test_bindings

import (
	"net/http"
	"time"

	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/bindings"
	"github.com/containers/libpod/pkg/bindings/images"
	"github.com/containers/libpod/pkg/bindings/manifests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman containers ", func() {
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
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		s.Kill()
		bt.cleanup()
	})

	It("create manifest", func() {
		// create manifest list without images
		id, err := manifests.Create(bt.conn, []string{"quay.io/libpod/foobar:latest"}, []string{}, nil)
		Expect(err).To(BeNil())
		list, err := manifests.Inspect(bt.conn, id)
		Expect(err).To(BeNil())
		Expect(len(list.Manifests)).To(BeZero())

		// creating a duplicate should fail as a 500
		_, err = manifests.Create(bt.conn, []string{"quay.io/libpod/foobar:latest"}, []string{}, nil)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))

		_, err = images.Remove(bt.conn, id, false)
		Expect(err).To(BeNil())

		// create manifest list with images
		id, err = manifests.Create(bt.conn, []string{"quay.io/libpod/foobar:latest"}, []string{alpine.name}, nil)
		Expect(err).To(BeNil())
		list, err = manifests.Inspect(bt.conn, id)
		Expect(err).To(BeNil())
		Expect(len(list.Manifests)).To(BeNumerically("==", 1))
	})

	It("inspect bogus manifest", func() {
		_, err := manifests.Inspect(bt.conn, "larry")
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))
	})

	It("add manifest", func() {
		// add to bogus should 404
		_, err := manifests.Add(bt.conn, "foobar", image.ManifestAddOpts{})
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		id, err := manifests.Create(bt.conn, []string{"quay.io/libpod/foobar:latest"}, []string{}, nil)
		Expect(err).To(BeNil())
		opts := image.ManifestAddOpts{Images: []string{alpine.name}}
		_, err = manifests.Add(bt.conn, id, opts)
		Expect(err).To(BeNil())
		list, err := manifests.Inspect(bt.conn, id)
		Expect(err).To(BeNil())
		Expect(len(list.Manifests)).To(BeNumerically("==", 1))

		// add bogus name to existing list should fail
		opts.Images = []string{"larry"}
		_, err = manifests.Add(bt.conn, id, opts)
		Expect(err).ToNot(BeNil())
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("remove manifest", func() {
		// removal on bogus manifest list should be 404
		_, err := manifests.Remove(bt.conn, "larry", "1234")
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		id, err := manifests.Create(bt.conn, []string{"quay.io/libpod/foobar:latest"}, []string{alpine.name}, nil)
		Expect(err).To(BeNil())
		data, err := manifests.Inspect(bt.conn, id)
		Expect(err).To(BeNil())
		Expect(len(data.Manifests)).To(BeNumerically("==", 1))

		// removal on a good manifest list with a bad digest should be 400
		_, err = manifests.Remove(bt.conn, id, "!234")
		Expect(err).ToNot(BeNil())
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusBadRequest))

		digest := data.Manifests[0].Digest.String()
		_, err = manifests.Remove(bt.conn, id, digest)
		Expect(err).To(BeNil())

		// removal on good manifest with good digest should work
		data, err = manifests.Inspect(bt.conn, id)
		Expect(err).To(BeNil())
		Expect(len(data.Manifests)).To(BeZero())
	})

	// There is NO annotate endpoint, this could never work.:w

	//It("annotate manifest", func() {
	//	id, err := manifests.Create(bt.conn, []string{"quay.io/libpod/foobar:latest"}, []string{}, nil)
	//	Expect(err).To(BeNil())
	//	opts := image.ManifestAddOpts{Images: []string{"docker.io/library/alpine:latest"}}
	//
	//	_, err = manifests.Add(bt.conn, id, opts)
	//	Expect(err).To(BeNil())
	//	data, err := manifests.Inspect(bt.conn, id)
	//	Expect(err).To(BeNil())
	//	Expect(len(data.Manifests)).To(BeNumerically("==", 1))
	//	digest := data.Manifests[0].Digest.String()
	//	annoOpts := image.ManifestAnnotateOpts{OS: "foo"}
	//	_, err = manifests.Annotate(bt.conn, id, digest, annoOpts)
	//	Expect(err).To(BeNil())
	//	list, err := manifests.Inspect(bt.conn, id)
	//	Expect(err).To(BeNil())
	//	Expect(len(list.Manifests)).To(BeNumerically("==", 1))
	//	Expect(list.Manifests[0].Platform.OS).To(Equal("foo"))
	//})

	It("push manifest", func() {
		Skip("TODO")
	})
})
