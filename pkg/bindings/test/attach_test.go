package test_bindings

import (
	"bytes"
	"time"

	"github.com/containers/libpod/pkg/bindings"
	"github.com/containers/libpod/pkg/bindings/containers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman containers attach", func() {
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
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		s.Kill()
		bt.cleanup()
	})

	It("attach", func() {
		name := "TopAttachTest"
		id, err := bt.RunTopContainer(&name, nil, nil)
		Expect(err).ShouldNot(HaveOccurred())

		tickTock := time.NewTimer(2 * time.Second)
		go func() {
			<-tickTock.C
			timeout := uint(5)
			err := containers.Stop(bt.conn, id, &timeout)
			if err != nil {
				GinkgoWriter.Write([]byte(err.Error()))
			}
		}()

		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		go func() {
			defer GinkgoRecover()

			err := containers.Attach(bt.conn, id, nil, &bindings.PTrue, &bindings.PTrue, &bindings.PTrue, stdout, stderr)
			Expect(err).ShouldNot(HaveOccurred())
		}()

		time.Sleep(5 * time.Second)

		// First character/First line of top output
		Expect(stdout.String()).Should(ContainSubstring("Mem: "))
	})
})
