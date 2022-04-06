package bindings_test

import (
	"runtime"
	"time"

	"github.com/containers/podman/v4/pkg/bindings/containers"
	"github.com/containers/podman/v4/pkg/bindings/images"
	"github.com/containers/podman/v4/pkg/bindings/system"
	"github.com/containers/podman/v4/pkg/specgen"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman info", func() {
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

	It("podman info", func() {
		info, err := system.Info(bt.conn, nil)
		Expect(err).To(BeNil())
		Expect(info.Host.Arch).To(Equal(runtime.GOARCH))
		Expect(info.Host.OS).To(Equal(runtime.GOOS))
		listOptions := new(images.ListOptions)
		i, err := images.List(bt.conn, listOptions.WithAll(true))
		Expect(err).To(BeNil())
		Expect(info.Store.ImageStore.Number).To(Equal(len(i)))
	})

	It("podman info container counts", func() {
		s := specgen.NewSpecGenerator(alpine.name, false)
		_, err := containers.CreateWithSpec(bt.conn, s, nil)
		Expect(err).To(BeNil())

		idPause, err := bt.RunTopContainer(nil, nil)
		Expect(err).To(BeNil())
		err = containers.Pause(bt.conn, idPause, nil)
		Expect(err).To(BeNil())

		idStop, err := bt.RunTopContainer(nil, nil)
		Expect(err).To(BeNil())
		err = containers.Stop(bt.conn, idStop, nil)
		Expect(err).To(BeNil())

		_, err = bt.RunTopContainer(nil, nil)
		Expect(err).To(BeNil())

		info, err := system.Info(bt.conn, nil)
		Expect(err).To(BeNil())

		Expect(info.Store.ContainerStore.Number).To(BeNumerically("==", 4))
		Expect(info.Store.ContainerStore.Paused).To(Equal(1))
		Expect(info.Store.ContainerStore.Stopped).To(Equal(2))
		Expect(info.Store.ContainerStore.Running).To(Equal(1))
	})
})
