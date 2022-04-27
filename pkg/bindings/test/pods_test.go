package bindings_test

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/bindings/pods"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/errorhandling"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/containers/podman/v4/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman pods", func() {
	var (
		bt     *bindingTest
		s      *gexec.Session
		newpod string
		err    error
	)

	BeforeEach(func() {
		bt = newBindingTest()
		newpod = "newpod"
		bt.RestoreImagesFromCache()
		bt.Podcreate(&newpod)
		s = bt.startAPIService()
		time.Sleep(1 * time.Second)
		err := bt.NewConnection()
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		s.Kill()
		bt.cleanup()
	})

	It("inspect pod", func() {
		// Inspect an invalid pod name
		_, err := pods.Inspect(bt.conn, "dummyname", nil)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Inspect an valid pod name
		response, err := pods.Inspect(bt.conn, newpod, nil)
		Expect(err).To(BeNil())
		Expect(response.Name).To(Equal(newpod))
	})

	// Test validates the list all api returns
	It("list pod", func() {
		// List all the pods in the current instance
		podSummary, err := pods.List(bt.conn, nil)
		Expect(err).To(BeNil())
		Expect(len(podSummary)).To(Equal(1))

		// Start the pod
		_, err = pods.Start(bt.conn, newpod, nil)
		Expect(err).To(BeNil())

		// Adding an alpine container to the existing pod
		_, err = bt.RunTopContainer(nil, &newpod)
		Expect(err).To(BeNil())
		podSummary, err = pods.List(bt.conn, nil)
		// Verify no errors.
		Expect(err).To(BeNil())
		// Verify number of containers in the pod.
		Expect(len(podSummary[0].Containers)).To(Equal(2))

		// Add multiple pods and verify them by name and size.
		var newpod2 string = "newpod2"
		bt.Podcreate(&newpod2)
		podSummary, err = pods.List(bt.conn, nil)
		Expect(err).To(BeNil(), "Error from pods.List")
		Expect(len(podSummary)).To(Equal(2))
		var names []string
		for _, i := range podSummary {
			names = append(names, i.Name)
		}
		Expect(StringInSlice(newpod, names)).To(BeTrue())
		Expect(StringInSlice("newpod2", names)).To(BeTrue())
	})

	// The test validates the list pod endpoint with passing filters as the params.
	It("List pods with filters", func() {
		newpod2 := "newpod2"
		bt.Podcreate(&newpod2)

		// Start the pod
		_, err = pods.Start(bt.conn, newpod, nil)
		Expect(err).To(BeNil())

		_, err = bt.RunTopContainer(nil, &newpod)
		Expect(err).To(BeNil())

		// Expected err with invalid filter params
		filters := make(map[string][]string)
		filters["dummy"] = []string{"dummy"}
		options := new(pods.ListOptions).WithFilters(filters)
		filteredPods, err := pods.List(bt.conn, options)
		Expect(err).ToNot(BeNil())
		Expect(len(filteredPods)).To(Equal(0), "len(filteredPods)")
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))

		// Expected empty response with invalid filters
		filters = make(map[string][]string)
		filters["name"] = []string{"dummy"}
		options = new(pods.ListOptions).WithFilters(filters)
		filteredPods, err = pods.List(bt.conn, options)
		Expect(err).To(BeNil())
		Expect(len(filteredPods)).To(BeNumerically("==", 0))

		// Validate list pod with name filter
		filters = make(map[string][]string)
		filters["name"] = []string{newpod2}
		options = new(pods.ListOptions).WithFilters(filters)
		filteredPods, err = pods.List(bt.conn, options)
		Expect(err).To(BeNil())
		Expect(len(filteredPods)).To(BeNumerically("==", 1))
		var names []string
		for _, i := range filteredPods {
			names = append(names, i.Name)
		}
		Expect(StringInSlice("newpod2", names)).To(BeTrue())

		// Validate list pod with id filter
		filters = make(map[string][]string)
		response, err := pods.Inspect(bt.conn, newpod, nil)
		Expect(err).To(BeNil())
		id := response.ID
		filters["id"] = []string{id}
		options = new(pods.ListOptions).WithFilters(filters)
		filteredPods, err = pods.List(bt.conn, options)
		Expect(err).To(BeNil())
		Expect(len(filteredPods)).To(BeNumerically("==", 1))
		names = names[:0]
		for _, i := range filteredPods {
			names = append(names, i.Name)
		}
		Expect(StringInSlice("newpod", names)).To(BeTrue())

		// Using multiple filters
		filters["name"] = []string{newpod}
		options = new(pods.ListOptions).WithFilters(filters)
		filteredPods, err = pods.List(bt.conn, options)
		Expect(err).To(BeNil())
		Expect(len(filteredPods)).To(BeNumerically("==", 1))
		names = names[:0]
		for _, i := range filteredPods {
			names = append(names, i.Name)
		}
		Expect(StringInSlice("newpod", names)).To(BeTrue())
	})

	// The test validates if the exists responds
	It("exists pod", func() {
		response, err := pods.Exists(bt.conn, "dummyName", nil)
		Expect(err).To(BeNil())
		Expect(response).To(BeFalse())

		// Should exit with no error and response should be true
		response, err = pods.Exists(bt.conn, "newpod", nil)
		Expect(err).To(BeNil())
		Expect(response).To(BeTrue())
	})

	// This test validates if All running containers within
	// each specified pod are paused and unpaused
	It("pause unpause pod", func() {
		// TODO fix this
		Skip("Pod behavior is jacked right now.")
		// Pause invalid container
		_, err := pods.Pause(bt.conn, "dummyName", nil)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Adding an alpine container to the existing pod
		_, err = bt.RunTopContainer(nil, &newpod)
		Expect(err).To(BeNil())

		// Binding needs to be modified to inspect the pod state.
		// Since we don't have a pod state we inspect the states of the containers within the pod.
		// Pause a valid container
		_, err = pods.Pause(bt.conn, newpod, nil)
		Expect(err).To(BeNil())
		response, err := pods.Inspect(bt.conn, newpod, nil)
		Expect(err).To(BeNil())
		Expect(response.State).To(Equal(define.PodStatePaused))
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStatePaused))
		}

		// Unpause a valid container
		_, err = pods.Unpause(bt.conn, newpod, nil)
		Expect(err).To(BeNil())
		response, err = pods.Inspect(bt.conn, newpod, nil)
		Expect(err).To(BeNil())
		Expect(response.State).To(Equal(define.PodStateRunning))
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStateRunning))
		}
	})

	It("start pod with port conflict", func() {
		randomport, err := utils.GetRandomPort()
		Expect(err).To(BeNil())

		portPublish := fmt.Sprintf("%d:%d", randomport, randomport)
		var podwithport string = "newpodwithport"
		bt.PodcreateAndExpose(&podwithport, &portPublish)

		// Start pod and expose port 12345
		_, err = pods.Start(bt.conn, podwithport, nil)
		Expect(err).To(BeNil())

		// Start another pod and expose same port 12345
		var podwithport2 string = "newpodwithport2"
		bt.PodcreateAndExpose(&podwithport2, &portPublish)

		_, err = pods.Start(bt.conn, podwithport2, nil)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusConflict))
		Expect(err).To(BeAssignableToTypeOf(&errorhandling.PodConflictErrorModel{}))
	})

	It("start stop restart pod", func() {
		// Start an invalid pod
		_, err = pods.Start(bt.conn, "dummyName", nil)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Stop an invalid pod
		_, err = pods.Stop(bt.conn, "dummyName", nil)
		Expect(err).ToNot(BeNil())
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Restart an invalid pod
		_, err = pods.Restart(bt.conn, "dummyName", nil)
		Expect(err).ToNot(BeNil())
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Start a valid pod and inspect status of each container
		_, err = pods.Start(bt.conn, newpod, nil)
		Expect(err).To(BeNil())

		response, err := pods.Inspect(bt.conn, newpod, nil)
		Expect(err).To(BeNil())
		Expect(response.State).To(Equal(define.PodStateRunning))
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStateRunning))
		}

		// Start an already running  pod
		_, err = pods.Start(bt.conn, newpod, nil)
		Expect(err).To(BeNil())

		// Stop the running pods
		_, err = pods.Stop(bt.conn, newpod, nil)
		Expect(err).To(BeNil())
		response, _ = pods.Inspect(bt.conn, newpod, nil)
		Expect(response.State).To(Equal(define.PodStateExited))
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStateExited))
		}

		// Stop an already stopped pod
		_, err = pods.Stop(bt.conn, newpod, nil)
		Expect(err).To(BeNil())

		_, err = pods.Restart(bt.conn, newpod, nil)
		Expect(err).To(BeNil())
		response, _ = pods.Inspect(bt.conn, newpod, nil)
		Expect(response.State).To(Equal(define.PodStateRunning))
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStateRunning))
		}
	})

	// Test to validate all the pods in the stopped/exited state are pruned successfully.
	It("prune pod", func() {
		// Add a new pod
		var newpod2 string = "newpod2"
		bt.Podcreate(&newpod2)
		// No pods pruned since no pod in exited state
		pruneResponse, err := pods.Prune(bt.conn, nil)
		Expect(err).To(BeNil())
		Expect(len(pruneResponse)).To(Equal(0), "len(pruneResponse)")
		podSummary, err := pods.List(bt.conn, nil)
		Expect(err).To(BeNil())
		Expect(len(podSummary)).To(Equal(2))

		// Prune only one pod which is in exited state.
		// Start then stop a pod.
		// pod moves to exited state one pod should be pruned now.
		_, err = pods.Start(bt.conn, newpod, nil)
		Expect(err).To(BeNil())
		_, err = pods.Stop(bt.conn, newpod, nil)
		Expect(err).To(BeNil())
		response, err := pods.Inspect(bt.conn, newpod, nil)
		Expect(err).To(BeNil())
		Expect(response.State).To(Equal(define.PodStateExited))
		pruneResponse, err = pods.Prune(bt.conn, nil)
		Expect(err).To(BeNil())
		Expect(len(pruneResponse)).To(Equal(1), "len(pruneResponse)")
		// Validate status and record pod id of pod to be pruned
		Expect(response.State).To(Equal(define.PodStateExited))
		podID := response.ID
		// Check if right pod was pruned
		Expect(len(pruneResponse)).To(Equal(1))
		Expect(pruneResponse[0].Id).To(Equal(podID))
		// One pod is pruned hence only one pod should be active.
		podSummary, err = pods.List(bt.conn, nil)
		Expect(err).To(BeNil())
		Expect(len(podSummary)).To(Equal(1))

		// Test prune multiple pods.
		bt.Podcreate(&newpod)
		_, err = pods.Start(bt.conn, newpod, nil)
		Expect(err).To(BeNil())
		_, err = pods.Start(bt.conn, newpod2, nil)
		Expect(err).To(BeNil())
		_, err = pods.Stop(bt.conn, newpod, nil)
		Expect(err).To(BeNil())
		response, err = pods.Inspect(bt.conn, newpod, nil)
		Expect(err).To(BeNil())
		Expect(response.State).To(Equal(define.PodStateExited))
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStateExited))
		}
		_, err = pods.Stop(bt.conn, newpod2, nil)
		Expect(err).To(BeNil())
		response, err = pods.Inspect(bt.conn, newpod2, nil)
		Expect(err).To(BeNil())
		Expect(response.State).To(Equal(define.PodStateExited))
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStateExited))
		}
		_, err = pods.Prune(bt.conn, nil)
		Expect(err).To(BeNil())
		podSummary, err = pods.List(bt.conn, nil)
		Expect(err).To(BeNil())
		Expect(len(podSummary)).To(Equal(0))
	})

	It("simple create pod", func() {
		ps := entities.PodSpec{PodSpecGen: specgen.PodSpecGenerator{InfraContainerSpec: &specgen.SpecGenerator{}}}
		ps.PodSpecGen.Name = "foobar"
		_, err := pods.CreatePodFromSpec(bt.conn, &ps)
		Expect(err).To(BeNil())

		exists, err := pods.Exists(bt.conn, "foobar", nil)
		Expect(err).To(BeNil())
		Expect(exists).To(BeTrue())
	})

	// Test validates the pod top bindings
	It("pod top", func() {
		var name string = "podA"

		bt.Podcreate(&name)
		_, err := pods.Start(bt.conn, name, nil)
		Expect(err).To(BeNil())

		// By name
		_, err = pods.Top(bt.conn, name, nil)
		Expect(err).To(BeNil())

		// With descriptors
		options := new(pods.TopOptions).WithDescriptors([]string{"user,pid,hpid"})
		output, err := pods.Top(bt.conn, name, options)
		Expect(err).To(BeNil())
		header := strings.Split(output[0], "\t")
		for _, d := range []string{"USER", "PID", "HPID"} {
			Expect(d).To(BeElementOf(header))
		}

		// With bogus ID
		_, err = pods.Top(bt.conn, "IdoNotExist", nil)
		Expect(err).ToNot(BeNil())

		// With bogus descriptors
		options = new(pods.TopOptions).WithDescriptors([]string{"Me,Neither"})
		_, err = pods.Top(bt.conn, name, options)
		Expect(err).ToNot(BeNil())
	})
})
