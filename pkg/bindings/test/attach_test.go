package test_bindings

import (
	"bytes"
	"fmt"
	"time"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/bindings"
	"github.com/containers/libpod/pkg/bindings/containers"
	"github.com/containers/libpod/pkg/specgen"
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

	It("can run top in container", func() {
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

			err := containers.Attach(bt.conn, id, nil, &bindings.PTrue, &bindings.PTrue, nil, stdout, stderr)
			Expect(err).ShouldNot(HaveOccurred())
		}()

		time.Sleep(5 * time.Second)
		// First character/First line of top output
		Expect(stdout.String()).Should(ContainSubstring("Mem: "))
	})

	It("can echo data via cat in container", func() {
		s := specgen.NewSpecGenerator(alpine.name, false)
		s.Name = "CatAttachTest"
		s.Terminal = true
		s.Command = []string{"/bin/cat"}
		ctnr, err := containers.CreateWithSpec(bt.conn, s)
		Expect(err).ShouldNot(HaveOccurred())

		err = containers.Start(bt.conn, ctnr.ID, nil)
		Expect(err).ShouldNot(HaveOccurred())

		wait := define.ContainerStateRunning
		_, err = containers.Wait(bt.conn, ctnr.ID, &wait)
		Expect(err).ShouldNot(HaveOccurred())

		tickTock := time.NewTimer(2 * time.Second)
		go func() {
			<-tickTock.C
			timeout := uint(5)
			err := containers.Stop(bt.conn, ctnr.ID, &timeout)
			if err != nil {
				GinkgoWriter.Write([]byte(err.Error()))
			}
		}()

		msg := "Hello, World"
		stdin := &bytes.Buffer{}
		stdin.WriteString(msg + "\n")

		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		go func() {
			defer GinkgoRecover()

			err := containers.Attach(bt.conn, ctnr.ID, nil, &bindings.PFalse, &bindings.PTrue, stdin, stdout, stderr)
			Expect(err).ShouldNot(HaveOccurred())
		}()

		time.Sleep(5 * time.Second)
		// Tty==true so we get echo'ed stdin + expected output
		Expect(stdout.String()).Should(Equal(fmt.Sprintf("%[1]s\r\n%[1]s\r\n", msg)))
		Expect(stderr.String()).Should(BeEmpty())
	})
})
