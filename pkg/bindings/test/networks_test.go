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

	It("podman prune unused networks with filters", func() {
		name := "foobar"
		opts := network.CreateOptions{
			Name: &name,
		}
		_, err = network.Create(connText, &opts)
		Expect(err).To(BeNil())

		// Invalid filters should return error
		filtersIncorrect := map[string][]string{
			"status": {"dummy"},
		}
		_, err = network.Prune(connText, new(network.PruneOptions).WithFilters(filtersIncorrect))
		Expect(err).ToNot(BeNil())

		// List filter params should not work with prune.
		filtersIncorrect = map[string][]string{
			"name": {name},
		}
		_, err = network.Prune(connText, new(network.PruneOptions).WithFilters(filtersIncorrect))
		Expect(err).ToNot(BeNil())

		// Mismatched label, correct filter params => no network should be pruned.
		filtersIncorrect = map[string][]string{
			"label": {"xyz"},
		}
		pruneResponse, err := network.Prune(connText, new(network.PruneOptions).WithFilters(filtersIncorrect))
		Expect(err).To(BeNil())
		Expect(len(pruneResponse)).To(Equal(0))

		// Mismatched until, correct filter params => no network should be pruned.
		filters := map[string][]string{
			"until": {"50"}, // January 1, 1970
		}
		pruneResponse, err = network.Prune(connText, new(network.PruneOptions).WithFilters(filters))
		Expect(err).To(BeNil())
		Expect(len(pruneResponse)).To(Equal(0))

		// Valid filter params => network should be pruned now.
		filters = map[string][]string{
			"until": {"5000000000"}, //June 11, 2128
		}
		pruneResponse, err = network.Prune(connText, new(network.PruneOptions).WithFilters(filters))
		Expect(err).To(BeNil())
		Expect(len(pruneResponse)).To(Equal(1))
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
