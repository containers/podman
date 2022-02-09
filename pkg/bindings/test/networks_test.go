package bindings_test

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/containers/common/libnetwork/types"
	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/bindings/containers"
	"github.com/containers/podman/v4/pkg/bindings/network"
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
		net := types.Network{
			Name: name,
		}
		_, err = network.Create(connText, &net)
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
			"until": {"5000000000"}, // June 11, 2128
		}
		pruneResponse, err = network.Prune(connText, new(network.PruneOptions).WithFilters(filters))
		Expect(err).To(BeNil())
		Expect(len(pruneResponse)).To(Equal(1))
	})

	It("create network", func() {
		// create a network with blank config should work
		_, err = network.Create(connText, nil)
		Expect(err).To(BeNil())

		name := "foobar"
		net := types.Network{
			Name: name,
		}

		report, err := network.Create(connText, &net)
		Expect(err).To(BeNil())
		Expect(report.Name).To(Equal(name))

		// create network with same name should 500
		_, err = network.Create(connText, &net)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusConflict))
	})

	It("inspect network", func() {
		name := "foobar"
		net := types.Network{
			Name: name,
		}
		_, err = network.Create(connText, &net)
		Expect(err).To(BeNil())
		data, err := network.Inspect(connText, name, nil)
		Expect(err).To(BeNil())
		Expect(data.Name).To(Equal(name))
	})

	It("list networks", func() {
		// create a bunch of named networks and make verify with list
		netNames := []string{"homer", "bart", "lisa", "maggie", "marge"}
		for i := 0; i < 5; i++ {
			net := types.Network{
				Name: netNames[i],
			}
			_, err = network.Create(connText, &net)
			Expect(err).To(BeNil())
		}
		list, err := network.List(connText, nil)
		Expect(err).To(BeNil())
		Expect(len(list)).To(BeNumerically(">=", 5))
		for _, n := range list {
			if n.Name != "podman" {
				Expect(StringInSlice(n.Name, netNames)).To(BeTrue())
			}
		}

		// list with bad filter should be 500
		filters := make(map[string][]string)
		filters["foobar"] = []string{"1234"}
		options := new(network.ListOptions).WithFilters(filters)
		_, err = network.List(connText, options)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))

		// filter list with success
		filters = make(map[string][]string)
		filters["name"] = []string{"homer"}
		options = new(network.ListOptions).WithFilters(filters)
		list, err = network.List(connText, options)
		Expect(err).To(BeNil())
		Expect(len(list)).To(BeNumerically("==", 1))
		Expect(list[0].Name).To(Equal("homer"))
	})

	It("remove network", func() {
		// removing a noName network should result in 404
		_, err := network.Remove(connText, "noName", nil)
		code, err := bindings.CheckResponseCode(err)
		Expect(err).To(BeNil())
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Removing an unused network should work
		name := "unused"
		net := types.Network{
			Name: name,
		}
		_, err = network.Create(connText, &net)
		Expect(err).To(BeNil())
		report, err := network.Remove(connText, name, nil)
		Expect(err).To(BeNil())
		Expect(report[0].Name).To(Equal(name))

		// Removing a network that is being used without force should be 500
		name = "used"
		net = types.Network{
			Name: name,
		}
		_, err = network.Create(connText, &net)
		Expect(err).To(BeNil())

		// Start container and wait
		container := "ntest"
		session := bt.runPodman([]string{"run", "-dt", fmt.Sprintf("--network=%s", name), "--name", container, alpine.name, "top"})
		session.Wait(45)
		Expect(session.ExitCode()).To(BeZero())

		_, err = network.Remove(connText, name, nil)
		code, err = bindings.CheckResponseCode(err)
		Expect(err).To(BeNil())
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))

		// Removing with a network in use with force should work with a stopped container
		err = containers.Stop(connText, container, new(containers.StopOptions).WithTimeout(0))
		Expect(err).To(BeNil())
		options := new(network.RemoveOptions).WithForce(true)
		report, err = network.Remove(connText, name, options)
		Expect(err).To(BeNil())
		Expect(report[0].Name).To(Equal(name))
	})
})
