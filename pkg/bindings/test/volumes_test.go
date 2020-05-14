package test_bindings

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/containers/libpod/pkg/bindings"
	"github.com/containers/libpod/pkg/bindings/containers"
	"github.com/containers/libpod/pkg/bindings/volumes"
	"github.com/containers/libpod/pkg/domain/entities"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman volumes", func() {
	var (
		//tempdir    string
		//err        error
		//podmanTest *PodmanTestIntegration
		bt       *bindingTest
		s        *gexec.Session
		connText context.Context
		err      error
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

	It("create volume", func() {
		// create a volume with blank config should work
		_, err := volumes.Create(connText, entities.VolumeCreateOptions{})
		Expect(err).To(BeNil())

		vcc := entities.VolumeCreateOptions{
			Name:    "foobar",
			Label:   nil,
			Options: nil,
		}
		vol, err := volumes.Create(connText, vcc)
		Expect(err).To(BeNil())
		Expect(vol.Name).To(Equal("foobar"))

		// create volume with same name should 500
		_, err = volumes.Create(connText, vcc)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("inspect volume", func() {
		vol, err := volumes.Create(connText, entities.VolumeCreateOptions{})
		Expect(err).To(BeNil())
		data, err := volumes.Inspect(connText, vol.Name)
		Expect(err).To(BeNil())
		Expect(data.Name).To(Equal(vol.Name))
	})

	It("remove volume", func() {
		// removing a bogus volume should result in 404
		err := volumes.Remove(connText, "foobar", nil)
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Removing an unused volume should work
		vol, err := volumes.Create(connText, entities.VolumeCreateOptions{})
		Expect(err).To(BeNil())
		err = volumes.Remove(connText, vol.Name, nil)
		Expect(err).To(BeNil())

		// Removing a volume that is being used without force should be 409
		vol, err = volumes.Create(connText, entities.VolumeCreateOptions{})
		Expect(err).To(BeNil())
		session := bt.runPodman([]string{"run", "-dt", "-v", fmt.Sprintf("%s:/foobar", vol.Name), "--name", "vtest", alpine.name, "top"})
		session.Wait(45)
		err = volumes.Remove(connText, vol.Name, nil)
		Expect(err).ToNot(BeNil())
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusConflict))

		// Removing with a volume in use with force should work with a stopped container
		zero := uint(0)
		err = containers.Stop(connText, "vtest", &zero)
		Expect(err).To(BeNil())
		err = volumes.Remove(connText, vol.Name, bindings.PTrue)
		Expect(err).To(BeNil())
	})

	It("list volumes", func() {
		// no volumes should be ok
		vols, err := volumes.List(connText, nil)
		Expect(err).To(BeNil())
		Expect(len(vols)).To(BeZero())

		// create a bunch of named volumes and make verify with list
		volNames := []string{"homer", "bart", "lisa", "maggie", "marge"}
		for i := 0; i < 5; i++ {
			_, err = volumes.Create(connText, entities.VolumeCreateOptions{Name: volNames[i]})
			Expect(err).To(BeNil())
		}
		vols, err = volumes.List(connText, nil)
		Expect(err).To(BeNil())
		Expect(len(vols)).To(BeNumerically("==", 5))
		for _, v := range vols {
			Expect(StringInSlice(v.Name, volNames)).To(BeTrue())
		}

		// list with bad filter should be 500
		filters := make(map[string][]string)
		filters["foobar"] = []string{"1234"}
		_, err = volumes.List(connText, filters)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))

		filters = make(map[string][]string)
		filters["name"] = []string{"homer"}
		vols, err = volumes.List(connText, filters)
		Expect(err).To(BeNil())
		Expect(len(vols)).To(BeNumerically("==", 1))
		Expect(vols[0].Name).To(Equal("homer"))
	})

	// TODO we need to add filtering to tests
	It("prune unused volume", func() {
		// Pruning when no volumes present should be ok
		_, err := volumes.Prune(connText)
		Expect(err).To(BeNil())

		// Removing an unused volume should work
		_, err = volumes.Create(connText, entities.VolumeCreateOptions{})
		Expect(err).To(BeNil())
		vols, err := volumes.Prune(connText)
		Expect(err).To(BeNil())
		Expect(len(vols)).To(BeNumerically("==", 1))

		_, err = volumes.Create(connText, entities.VolumeCreateOptions{Name: "homer"})
		Expect(err).To(BeNil())
		_, err = volumes.Create(connText, entities.VolumeCreateOptions{})
		Expect(err).To(BeNil())
		session := bt.runPodman([]string{"run", "-dt", "-v", fmt.Sprintf("%s:/homer", "homer"), "--name", "vtest", alpine.name, "top"})
		session.Wait(45)
		vols, err = volumes.Prune(connText)
		Expect(err).To(BeNil())
		Expect(len(vols)).To(BeNumerically("==", 1))
		_, err = volumes.Inspect(connText, "homer")
		Expect(err).To(BeNil())
	})

})
