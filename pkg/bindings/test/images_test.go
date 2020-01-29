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
		// Inspect by ID
		//Inspect by long name should work, it doesnt (yet) i think it needs to be html escaped
		//_, err = images.GetImage(connText, alpine, nil)
		//Expect(err).To(BeNil())
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

})
