package bindings_test

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/bindings/secrets"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman secrets", func() {
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
		Expect(err).To(BeNil())
	})

	AfterEach(func() {

		s.Kill()
		bt.cleanup()
	})

	It("create secret", func() {
		r := strings.NewReader("mysecret")
		name := "mysecret"
		opts := &secrets.CreateOptions{
			Name: &name,
		}
		_, err := secrets.Create(connText, r, opts)
		Expect(err).To(BeNil())

		// should not be allowed to create duplicate secret name
		_, err = secrets.Create(connText, r, opts)
		Expect(err).To(Not(BeNil()))
	})

	It("inspect secret", func() {
		r := strings.NewReader("mysecret")
		name := "mysecret"
		opts := &secrets.CreateOptions{
			Name: &name,
		}
		_, err := secrets.Create(connText, r, opts)
		Expect(err).To(BeNil())

		data, err := secrets.Inspect(connText, name, nil)
		Expect(err).To(BeNil())
		Expect(data.Spec.Name).To(Equal(name))

		// inspecting non-existent secret should fail
		_, err = secrets.Inspect(connText, "notasecret", nil)
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))
	})

	It("list secret", func() {
		r := strings.NewReader("mysecret")
		name := "mysecret"
		opts := &secrets.CreateOptions{
			Name: &name,
		}
		_, err := secrets.Create(connText, r, opts)
		Expect(err).To(BeNil())

		data, err := secrets.List(connText, nil)
		Expect(err).To(BeNil())
		Expect(data[0].Spec.Name).To(Equal(name))
	})

	It("list multiple secret", func() {
		r := strings.NewReader("mysecret")
		name := "mysecret"
		opts := &secrets.CreateOptions{
			Name: &name,
		}
		_, err := secrets.Create(connText, r, opts)
		Expect(err).To(BeNil())

		r2 := strings.NewReader("mysecret2")
		name2 := "mysecret2"
		opts2 := &secrets.CreateOptions{
			Name: &name2,
		}
		_, err = secrets.Create(connText, r2, opts2)
		Expect(err).To(BeNil())

		data, err := secrets.List(connText, nil)
		Expect(err).To(BeNil())
		Expect(len(data)).To(Equal(2))
	})

	It("list no secrets", func() {
		data, err := secrets.List(connText, nil)
		Expect(err).To(BeNil())
		Expect(len(data)).To(Equal(0))
	})

	It("remove secret", func() {
		r := strings.NewReader("mysecret")
		name := "mysecret"
		opts := &secrets.CreateOptions{
			Name: &name,
		}
		_, err := secrets.Create(connText, r, opts)
		Expect(err).To(BeNil())

		err = secrets.Remove(connText, name)
		Expect(err).To(BeNil())

		// removing non-existent secret should fail
		err = secrets.Remove(connText, "nosecret")
		Expect(err).To(Not(BeNil()))
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))
	})

})
