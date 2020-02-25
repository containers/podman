package test_bindings

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/containers/libpod/pkg/bindings"
	"github.com/containers/libpod/pkg/bindings/containers"
	"github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman containers ", func() {
	var (
		bt        *bindingTest
		s         *gexec.Session
		connText  context.Context
		err       error
		falseFlag bool = false
		trueFlag  bool = true
	)

	BeforeEach(func() {
		bt = newBindingTest()
		bt.RestoreImagesFromCache()
		s = bt.startAPIService()
		time.Sleep(1 * time.Second)
		connText, err = bindings.NewConnection(context.Background(), bt.sock)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		s.Kill()
		bt.cleanup()
	})

	It("podman pause a bogus container", func() {
		// Pausing bogus container should return 404
		err = containers.Pause(connText, "foobar")
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))
	})

	It("podman unpause a bogus container", func() {
		// Unpausing bogus container should return 404
		err = containers.Unpause(connText, "foobar")
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))
	})

	It("podman pause a running container by name", func() {
		// Pausing by name should work
		var name = "top"
		bt.RunTopContainer(&name, &falseFlag, nil)
		err := containers.Pause(connText, name)
		Expect(err).To(BeNil())

		// Ensure container is paused
		data, err := containers.Inspect(connText, name, nil)
		Expect(err).To(BeNil())
		Expect(data.State.Status).To(Equal("paused"))
	})

	It("podman pause a running container by id", func() {
		// Pausing by id should work
		var name = "top"
		bt.RunTopContainer(&name, &falseFlag, nil)
		data, err := containers.Inspect(connText, name, nil)
		Expect(err).To(BeNil())
		err = containers.Pause(connText, data.ID)
		Expect(err).To(BeNil())

		// Ensure container is paused
		data, err = containers.Inspect(connText, data.ID, nil)
		Expect(data.State.Status).To(Equal("paused"))
	})

	It("podman unpause a running container by name", func() {
		// Unpausing by name should work
		var name = "top"
		bt.RunTopContainer(&name, &falseFlag, nil)
		err := containers.Pause(connText, name)
		Expect(err).To(BeNil())
		err = containers.Unpause(connText, name)
		Expect(err).To(BeNil())

		// Ensure container is unpaused
		data, err := containers.Inspect(connText, name, nil)
		Expect(data.State.Status).To(Equal("running"))
	})

	It("podman unpause a running container by ID", func() {
		// Unpausing by ID should work
		var name = "top"
		bt.RunTopContainer(&name, &falseFlag, nil)
		// Pause by name
		err := containers.Pause(connText, name)
		data, err := containers.Inspect(connText, name, nil)
		Expect(err).To(BeNil())
		err = containers.Unpause(connText, data.ID)
		Expect(err).To(BeNil())

		// Ensure container is unpaused
		data, err = containers.Inspect(connText, name, nil)
		Expect(data.State.Status).To(Equal("running"))
	})

	It("podman pause a paused container by name", func() {
		// Pausing a paused container by name should fail
		var name = "top"
		bt.RunTopContainer(&name, &falseFlag, nil)
		err := containers.Pause(connText, name)
		Expect(err).To(BeNil())
		err = containers.Pause(connText, name)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("podman pause a paused container by id", func() {
		// Pausing a paused container by id should fail
		var name = "top"
		bt.RunTopContainer(&name, &falseFlag, nil)
		data, err := containers.Inspect(connText, name, nil)
		Expect(err).To(BeNil())
		err = containers.Pause(connText, data.ID)
		Expect(err).To(BeNil())
		err = containers.Pause(connText, data.ID)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("podman pause a stopped container by name", func() {
		// Pausing a stopped container by name should fail
		var name = "top"
		bt.RunTopContainer(&name, &falseFlag, nil)
		err := containers.Stop(connText, name, nil)
		Expect(err).To(BeNil())
		err = containers.Pause(connText, name)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("podman pause a stopped container by id", func() {
		// Pausing a stopped container by id should fail
		var name = "top"
		bt.RunTopContainer(&name, &falseFlag, nil)
		data, err := containers.Inspect(connText, name, nil)
		err = containers.Stop(connText, data.ID, nil)
		Expect(err).To(BeNil())
		err = containers.Pause(connText, data.ID)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("podman remove a paused container by id without force", func() {
		// Removing a paused container without force should fail
		var name = "top"
		bt.RunTopContainer(&name, &falseFlag, nil)
		data, err := containers.Inspect(connText, name, nil)
		Expect(err).To(BeNil())
		err = containers.Pause(connText, data.ID)
		Expect(err).To(BeNil())
		err = containers.Remove(connText, data.ID, &falseFlag, &falseFlag)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("podman remove a paused container by id with force", func() {
		// FIXME: Skip on F31 and later
		host := utils.GetHostDistributionInfo()
		osVer, err := strconv.Atoi(host.Version)
		Expect(err).To(BeNil())
		if host.Distribution == "fedora" && osVer >= 31 {
			Skip("FIXME: https://github.com/containers/libpod/issues/5325")
		}

		// Removing a paused container with force should work
		var name = "top"
		bt.RunTopContainer(&name, &falseFlag, nil)
		data, err := containers.Inspect(connText, name, nil)
		Expect(err).To(BeNil())
		err = containers.Pause(connText, data.ID)
		Expect(err).To(BeNil())
		err = containers.Remove(connText, data.ID, &trueFlag, &falseFlag)
		Expect(err).To(BeNil())
	})

	It("podman stop a paused container by name", func() {
		// Stopping a paused container by name should fail
		var name = "top"
		bt.RunTopContainer(&name, &falseFlag, nil)
		err := containers.Pause(connText, name)
		Expect(err).To(BeNil())
		err = containers.Stop(connText, name, nil)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("podman stop a paused container by id", func() {
		// Stopping a paused container by id should fail
		var name = "top"
		bt.RunTopContainer(&name, &falseFlag, nil)
		data, err := containers.Inspect(connText, name, nil)
		Expect(err).To(BeNil())
		err = containers.Pause(connText, data.ID)
		Expect(err).To(BeNil())
		err = containers.Stop(connText, data.ID, nil)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("podman stop a running container by name", func() {
		// Stopping a running container by name should work
		var name = "top"
		bt.RunTopContainer(&name, &falseFlag, nil)
		err := containers.Stop(connText, name, nil)
		Expect(err).To(BeNil())

		// Ensure container is stopped
		data, err := containers.Inspect(connText, name, nil)
		Expect(err).To(BeNil())
		Expect(data.State.Status).To(Equal("exited"))
	})

	It("podman stop a running container by ID", func() {
		// Stopping a running container by ID should work
		var name = "top"
		bt.RunTopContainer(&name, &falseFlag, nil)
		data, err := containers.Inspect(connText, name, nil)
		Expect(err).To(BeNil())
		err = containers.Stop(connText, data.ID, nil)
		Expect(err).To(BeNil())

		// Ensure container is stopped
		data, err = containers.Inspect(connText, name, nil)
		Expect(err).To(BeNil())
		Expect(data.State.Status).To(Equal("exited"))
	})

})
