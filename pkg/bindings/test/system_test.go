package test_bindings

import (
	"time"

	"github.com/containers/libpod/pkg/bindings"
	"github.com/containers/libpod/pkg/bindings/containers"
	"github.com/containers/libpod/pkg/bindings/pods"
	"github.com/containers/libpod/pkg/bindings/system"
	"github.com/containers/libpod/pkg/bindings/volumes"
	"github.com/containers/libpod/pkg/domain/entities"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman system", func() {
	var (
		bt     *bindingTest
		s      *gexec.Session
		newpod string
	)

	BeforeEach(func() {
		bt = newBindingTest()
		bt.RestoreImagesFromCache()
		newpod = "newpod"
		bt.Podcreate(&newpod)
		s = bt.startAPIService()
		time.Sleep(1 * time.Second)
		err := bt.NewConnection()
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		s.Kill()
		bt.cleanup()
	})

	It("podman events", func() {
		eChan := make(chan entities.Event, 1)
		var messages []entities.Event
		cancelChan := make(chan bool, 1)
		go func() {
			for e := range eChan {
				messages = append(messages, e)
			}
		}()
		go func() {
			system.Events(bt.conn, eChan, cancelChan, nil, nil, nil)
		}()

		_, err := bt.RunTopContainer(nil, nil, nil)
		Expect(err).To(BeNil())
		cancelChan <- true
		Expect(len(messages)).To(BeNumerically("==", 3))
	})

	It("podman system prune - pod,container stopped", func() {
		// Start and stop a pod to enter in exited state.
		_, err := pods.Start(bt.conn, newpod)
		Expect(err).To(BeNil())
		_, err = pods.Stop(bt.conn, newpod, nil)
		Expect(err).To(BeNil())
		// Start and stop a container to enter in exited state.
		var name = "top"
		_, err = bt.RunTopContainer(&name, bindings.PFalse, nil)
		Expect(err).To(BeNil())
		err = containers.Stop(bt.conn, name, nil)
		Expect(err).To(BeNil())

		systemPruneResponse, err := system.Prune(bt.conn, bindings.PTrue, bindings.PFalse)
		Expect(err).To(BeNil())
		Expect(len(systemPruneResponse.PodPruneReport)).To(Equal(1))
		Expect(len(systemPruneResponse.ContainerPruneReport.ID)).To(Equal(1))
		Expect(len(systemPruneResponse.ImagePruneReport.Report.Id)).
			To(BeNumerically(">", 0))
		Expect(systemPruneResponse.ImagePruneReport.Report.Id).
			To(ContainElement("docker.io/library/alpine:latest"))
		Expect(len(systemPruneResponse.VolumePruneReport)).To(Equal(0))
	})

	It("podman system prune running alpine container", func() {
		// Start and stop a pod to enter in exited state.
		_, err := pods.Start(bt.conn, newpod)
		Expect(err).To(BeNil())
		_, err = pods.Stop(bt.conn, newpod, nil)
		Expect(err).To(BeNil())

		// Start and stop a container to enter in exited state.
		var name = "top"
		_, err = bt.RunTopContainer(&name, bindings.PFalse, nil)
		Expect(err).To(BeNil())
		err = containers.Stop(bt.conn, name, nil)
		Expect(err).To(BeNil())

		// Start container and leave in running
		var name2 = "top2"
		_, err = bt.RunTopContainer(&name2, bindings.PFalse, nil)
		Expect(err).To(BeNil())

		// Adding an unused volume
		_, err = volumes.Create(bt.conn, entities.VolumeCreateOptions{})
		Expect(err).To(BeNil())

		systemPruneResponse, err := system.Prune(bt.conn, bindings.PTrue, bindings.PFalse)
		Expect(err).To(BeNil())
		Expect(len(systemPruneResponse.PodPruneReport)).To(Equal(1))
		Expect(len(systemPruneResponse.ContainerPruneReport.ID)).To(Equal(1))
		Expect(len(systemPruneResponse.ImagePruneReport.Report.Id)).
			To(BeNumerically(">", 0))
		// Alpine image should not be pruned as used by running container
		Expect(systemPruneResponse.ImagePruneReport.Report.Id).
			ToNot(ContainElement("docker.io/library/alpine:latest"))
		// Though unsed volume is available it should not be pruned as flag set to false.
		Expect(len(systemPruneResponse.VolumePruneReport)).To(Equal(0))
	})

	It("podman system prune running alpine container volume prune", func() {
		// Start a pod and leave it running
		_, err := pods.Start(bt.conn, newpod)
		Expect(err).To(BeNil())

		// Start and stop a container to enter in exited state.
		var name = "top"
		_, err = bt.RunTopContainer(&name, bindings.PFalse, nil)
		Expect(err).To(BeNil())
		err = containers.Stop(bt.conn, name, nil)
		Expect(err).To(BeNil())

		// Start second container and leave in running
		var name2 = "top2"
		_, err = bt.RunTopContainer(&name2, bindings.PFalse, nil)
		Expect(err).To(BeNil())

		// Adding an unused volume should work
		_, err = volumes.Create(bt.conn, entities.VolumeCreateOptions{})
		Expect(err).To(BeNil())

		systemPruneResponse, err := system.Prune(bt.conn, bindings.PTrue, bindings.PTrue)
		Expect(err).To(BeNil())
		Expect(len(systemPruneResponse.PodPruneReport)).To(Equal(0))
		Expect(len(systemPruneResponse.ContainerPruneReport.ID)).To(Equal(1))
		Expect(len(systemPruneResponse.ImagePruneReport.Report.Id)).
			To(BeNumerically(">", 0))
		// Alpine image should not be pruned as used by running container
		Expect(systemPruneResponse.ImagePruneReport.Report.Id).
			ToNot(ContainElement("docker.io/library/alpine:latest"))
		// Volume should be pruned now as flag set true
		Expect(len(systemPruneResponse.VolumePruneReport)).To(Equal(1))
	})
})
