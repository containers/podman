package test_bindings

import (
	"time"

	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/bindings"
	"github.com/containers/libpod/pkg/bindings/containers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman containers exec", func() {
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

	It("Podman exec create makes an exec session", func() {
		name := "testCtr"
		cid, err := bt.RunTopContainer(&name, bindings.PFalse, nil)
		Expect(err).To(BeNil())

		execConfig := new(handlers.ExecCreateConfig)
		execConfig.Cmd = []string{"echo", "hello world"}

		sessionID, err := containers.ExecCreate(bt.conn, name, execConfig)
		Expect(err).To(BeNil())
		Expect(sessionID).To(Not(Equal("")))

		inspectOut, err := containers.ExecInspect(bt.conn, sessionID)
		Expect(err).To(BeNil())
		Expect(inspectOut.ContainerID).To(Equal(cid))
		Expect(inspectOut.ProcessConfig.Entrypoint).To(Equal("echo"))
		Expect(len(inspectOut.ProcessConfig.Arguments)).To(Equal(1))
		Expect(inspectOut.ProcessConfig.Arguments[0]).To(Equal("hello world"))
	})

	It("Podman exec create with bad command fails", func() {
		name := "testCtr"
		_, err := bt.RunTopContainer(&name, bindings.PFalse, nil)
		Expect(err).To(BeNil())

		execConfig := new(handlers.ExecCreateConfig)

		_, err = containers.ExecCreate(bt.conn, name, execConfig)
		Expect(err).To(Not(BeNil()))
	})

	It("Podman exec create with invalid container fails", func() {
		execConfig := new(handlers.ExecCreateConfig)
		execConfig.Cmd = []string{"echo", "hello world"}

		_, err := containers.ExecCreate(bt.conn, "doesnotexist", execConfig)
		Expect(err).To(Not(BeNil()))
	})

	It("Podman exec inspect on invalid session fails", func() {
		_, err := containers.ExecInspect(bt.conn, "0000000000000000000000000000000000000000000000000000000000000000")
		Expect(err).To(Not(BeNil()))
	})
})
