package test_bindings

import (
	"time"

	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/bindings/system"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman system", func() {
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

	It("podman events", func() {
		eChan := make(chan handlers.Event, 1)
		var messages []handlers.Event
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
})
