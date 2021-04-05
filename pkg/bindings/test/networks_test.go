package test_bindings

import (
	"context"
	"net/http"
	"time"

	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/podman/v3/pkg/bindings/network"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman networks", func() {
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
		_, err = network.Prune(connText, &network.PruneOptions{})
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		s.Kill()
		bt.cleanup()
	})

	It("create network", func() {
		// create a network with blank config should work
		_, err = network.Create(connText, &network.CreateOptions{})
		Expect(err).To(BeNil())

		name := "foobar"
		opts := network.CreateOptions{
			Name: &name,
		}

		report, err := network.Create(connText, &opts)
		Expect(err).To(BeNil())
		Expect(report.Filename).To(ContainSubstring(name))

		// create network with same name should 500
		_, err = network.Create(connText, &opts)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("inspect network", func() {
		name := "foobar"
		opts := network.CreateOptions{
			Name: &name,
		}
		_, err = network.Create(connText, &opts)
		Expect(err).To(BeNil())
		data, err := network.Inspect(connText, name, nil)
		Expect(err).To(BeNil())
		Expect(data[0]["name"]).To(Equal(name))
	})
})
