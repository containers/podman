package bindings_test

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/bindings/containers"
	"github.com/containers/podman/v4/pkg/bindings/volumes"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/entities/reports"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman volumes", func() {
	var (
		bt       *bindingTest
		s        *gexec.Session
		connText context.Context
		err      error
	)

	BeforeEach(func() {
		bt = newBindingTest()
		bt.RestoreImagesFromCache()
		s = bt.startAPIService()
		time.Sleep(1 * time.Second)
		connText, err = bindings.NewConnection(context.Background(), bt.sock)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		s.Kill()
		bt.cleanup()
	})

	It("create volume", func() {
		// create a volume with blank config should work
		_, err := volumes.Create(connText, entities.VolumeCreateOptions{}, nil)
		Expect(err).ToNot(HaveOccurred())

		vcc := entities.VolumeCreateOptions{
			Name:    "foobar",
			Label:   nil,
			Options: nil,
		}
		vol, err := volumes.Create(connText, vcc, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(vol.Name).To(Equal("foobar"))

		// create volume with same name should 500
		_, err = volumes.Create(connText, vcc, nil)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("inspect volume", func() {
		vol, err := volumes.Create(connText, entities.VolumeCreateOptions{}, nil)
		Expect(err).ToNot(HaveOccurred())
		data, err := volumes.Inspect(connText, vol.Name, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(data.Name).To(Equal(vol.Name))
	})

	It("remove volume", func() {
		// removing a bogus volume should result in 404
		err := volumes.Remove(connText, "foobar", nil)
		code, err := bindings.CheckResponseCode(err)
		Expect(err).ToNot(HaveOccurred())
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Removing an unused volume should work
		vol, err := volumes.Create(connText, entities.VolumeCreateOptions{}, nil)
		Expect(err).ToNot(HaveOccurred())
		err = volumes.Remove(connText, vol.Name, nil)
		Expect(err).ToNot(HaveOccurred())

		// Removing a volume that is being used without force should be 409
		vol, err = volumes.Create(connText, entities.VolumeCreateOptions{}, nil)
		Expect(err).ToNot(HaveOccurred())
		session := bt.runPodman([]string{"run", "-dt", "-v", fmt.Sprintf("%s:/foobar", vol.Name), "--name", "vtest", alpine.name, "top"})
		session.Wait(45)
		Expect(session.ExitCode()).To(BeZero())

		err = volumes.Remove(connText, vol.Name, nil)
		Expect(err).To(HaveOccurred())
		code, err = bindings.CheckResponseCode(err)
		Expect(err).ToNot(HaveOccurred())
		Expect(code).To(BeNumerically("==", http.StatusConflict))

		// Removing with a volume in use with force should work with a stopped container
		err = containers.Stop(connText, "vtest", new(containers.StopOptions).WithTimeout(0))
		Expect(err).ToNot(HaveOccurred())
		options := new(volumes.RemoveOptions).WithForce(true)
		err = volumes.Remove(connText, vol.Name, options)
		Expect(err).ToNot(HaveOccurred())
	})

	It("list volumes", func() {
		// no volumes should be ok
		vols, err := volumes.List(connText, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(vols).To(BeEmpty())

		// create a bunch of named volumes and make verify with list
		volNames := []string{"homer", "bart", "lisa", "maggie", "marge"}
		for i := 0; i < 5; i++ {
			_, err = volumes.Create(connText, entities.VolumeCreateOptions{Name: volNames[i]}, nil)
			Expect(err).ToNot(HaveOccurred())
		}
		vols, err = volumes.List(connText, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(vols).To(HaveLen(5))
		for _, v := range vols {
			Expect(StringInSlice(v.Name, volNames)).To(BeTrue())
		}

		// list with bad filter should be 500
		filters := make(map[string][]string)
		filters["foobar"] = []string{"1234"}
		options := new(volumes.ListOptions).WithFilters(filters)
		_, err = volumes.List(connText, options)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))

		filters = make(map[string][]string)
		filters["name"] = []string{"homer"}
		options = new(volumes.ListOptions).WithFilters(filters)
		vols, err = volumes.List(connText, options)
		Expect(err).ToNot(HaveOccurred())
		Expect(vols).To(HaveLen(1))
		Expect(vols[0].Name).To(Equal("homer"))
	})

	It("prune unused volume", func() {
		// Pruning when no volumes present should be ok
		_, err := volumes.Prune(connText, nil)
		Expect(err).ToNot(HaveOccurred())

		// Removing an unused volume should work
		_, err = volumes.Create(connText, entities.VolumeCreateOptions{}, nil)
		Expect(err).ToNot(HaveOccurred())
		vols, err := volumes.Prune(connText, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(vols).To(HaveLen(1))

		_, err = volumes.Create(connText, entities.VolumeCreateOptions{Name: "homer"}, nil)
		Expect(err).ToNot(HaveOccurred())
		_, err = volumes.Create(connText, entities.VolumeCreateOptions{}, nil)
		Expect(err).ToNot(HaveOccurred())
		session := bt.runPodman([]string{"run", "-dt", "-v", fmt.Sprintf("%s:/homer", "homer"), "--name", "vtest", alpine.name, "top"})
		session.Wait(45)
		vols, err = volumes.Prune(connText, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(reports.PruneReportsIds(vols)).To(HaveLen(1))
		_, err = volumes.Inspect(connText, "homer", nil)
		Expect(err).ToNot(HaveOccurred())

		// Removing volume with non matching filter shouldn't prune any volumes
		filters := make(map[string][]string)
		filters["label"] = []string{"label1=idontmatch"}
		_, err = volumes.Create(connText, entities.VolumeCreateOptions{Label: map[string]string{
			"label1": "value1",
		}}, nil)
		Expect(err).ToNot(HaveOccurred())
		options := new(volumes.PruneOptions).WithFilters(filters)
		vols, err = volumes.Prune(connText, options)
		Expect(err).ToNot(HaveOccurred())
		Expect(vols).To(BeEmpty())
		vol2, err := volumes.Create(connText, entities.VolumeCreateOptions{Label: map[string]string{
			"label1": "value2",
		}}, nil)
		Expect(err).ToNot(HaveOccurred())
		_, err = volumes.Create(connText, entities.VolumeCreateOptions{Label: map[string]string{
			"label1": "value3",
		}}, nil)
		Expect(err).ToNot(HaveOccurred())

		// Removing volume with matching filter label and value should remove specific entry
		filters = make(map[string][]string)
		filters["label"] = []string{"label1=value2"}
		options = new(volumes.PruneOptions).WithFilters(filters)
		vols, err = volumes.Prune(connText, options)
		Expect(err).ToNot(HaveOccurred())
		Expect(vols).To(HaveLen(1))
		Expect(vols[0].Id).To(Equal(vol2.Name))

		// Removing volumes with matching filter label should remove all matching volumes
		filters = make(map[string][]string)
		filters["label"] = []string{"label1"}
		options = new(volumes.PruneOptions).WithFilters(filters)
		vols, err = volumes.Prune(connText, options)
		Expect(err).ToNot(HaveOccurred())
		Expect(vols).To(HaveLen(2))
	})

})
