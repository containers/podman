package test_bindings

import (
	"net/http"
	"strings"
	"time"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/bindings"
	"github.com/containers/libpod/pkg/bindings/pods"
	"github.com/containers/libpod/pkg/specgen"
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
		//Inspect an invalid pod name
		_, err := pods.Inspect(bt.conn, "dummyname")
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		//Inspect an valid pod name
		response, err := pods.Inspect(bt.conn, newpod)
		Expect(err).To(BeNil())
		Expect(response.Name).To(Equal(newpod))
	})

	// Test validates the list all api returns
	It("list pod", func() {
		//List all the pods in the current instance
		podSummary, err := pods.List(bt.conn, nil)
		Expect(err).To(BeNil())
		Expect(len(podSummary)).To(Equal(1))

		// Start the pod
		_, err = pods.Start(bt.conn, newpod)
		Expect(err).To(BeNil())

		// Adding an alpine container to the existing pod
		_, err = bt.RunTopContainer(nil, bindings.PTrue, &newpod)
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
		_, err = pods.Start(bt.conn, newpod)
		Expect(err).To(BeNil())

		_, err = bt.RunTopContainer(nil, bindings.PTrue, &newpod)
		Expect(err).To(BeNil())

		// Expected err with invalid filter params
		filters := make(map[string][]string)
		filters["dummy"] = []string{"dummy"}
		filteredPods, err := pods.List(bt.conn, filters)
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))

		// Expected empty response with invalid filters
		filters = make(map[string][]string)
		filters["name"] = []string{"dummy"}
		filteredPods, err = pods.List(bt.conn, filters)
		Expect(err).To(BeNil())
		Expect(len(filteredPods)).To(BeNumerically("==", 0))

		// Validate list pod with name filter
		filters = make(map[string][]string)
		filters["name"] = []string{newpod2}
		filteredPods, err = pods.List(bt.conn, filters)
		Expect(err).To(BeNil())
		Expect(len(filteredPods)).To(BeNumerically("==", 1))
		var names []string
		for _, i := range filteredPods {
			names = append(names, i.Name)
		}
		Expect(StringInSlice("newpod2", names)).To(BeTrue())

		// Validate list pod with id filter
		filters = make(map[string][]string)
		response, err := pods.Inspect(bt.conn, newpod)
		Expect(err).To(BeNil())
		id := response.ID
		filters["id"] = []string{id}
		filteredPods, err = pods.List(bt.conn, filters)
		Expect(err).To(BeNil())
		Expect(len(filteredPods)).To(BeNumerically("==", 1))
		names = names[:0]
		for _, i := range filteredPods {
			names = append(names, i.Name)
		}
		Expect(StringInSlice("newpod", names)).To(BeTrue())

		// Using multiple filters
		filters["name"] = []string{newpod}
		filteredPods, err = pods.List(bt.conn, filters)
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
		response, err := pods.Exists(bt.conn, "dummyName")
		Expect(err).To(BeNil())
		Expect(response).To(BeFalse())

		// Should exit with no error and response should be true
		response, err = pods.Exists(bt.conn, "newpod")
		Expect(err).To(BeNil())
		Expect(response).To(BeTrue())
	})

	// This test validates if All running containers within
	// each specified pod are paused and unpaused
	It("pause upause pod", func() {
		// TODO fix this
		Skip("Pod behavior is jacked right now.")
		// Pause invalid container
		_, err := pods.Pause(bt.conn, "dummyName")
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Adding an alpine container to the existing pod
		_, err = bt.RunTopContainer(nil, bindings.PTrue, &newpod)
		Expect(err).To(BeNil())

		// Binding needs to be modified to inspect the pod state.
		// Since we don't have a pod state we inspect the states of the containers within the pod.
		// Pause a valid container
		_, err = pods.Pause(bt.conn, newpod)
		Expect(err).To(BeNil())
		response, err := pods.Inspect(bt.conn, newpod)
		Expect(err).To(BeNil())
		Expect(response.State).To(Equal(define.PodStatePaused))
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStatePaused))
		}

		// Unpause a valid container
		_, err = pods.Unpause(bt.conn, newpod)
		Expect(err).To(BeNil())
		response, err = pods.Inspect(bt.conn, newpod)
		Expect(err).To(BeNil())
		Expect(response.State).To(Equal(define.PodStateRunning))
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStateRunning))
		}
	})

	It("start stop restart pod", func() {
		// Start an invalid pod
		_, err = pods.Start(bt.conn, "dummyName")
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Stop an invalid pod
		_, err = pods.Stop(bt.conn, "dummyName", nil)
		Expect(err).ToNot(BeNil())
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Restart an invalid pod
		_, err = pods.Restart(bt.conn, "dummyName")
		Expect(err).ToNot(BeNil())
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Start a valid pod and inspect status of each container
		_, err = pods.Start(bt.conn, newpod)
		Expect(err).To(BeNil())

		response, err := pods.Inspect(bt.conn, newpod)
		Expect(err).To(BeNil())
		Expect(response.State).To(Equal(define.PodStateRunning))
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStateRunning))
		}

		// Start an already running  pod
		_, err = pods.Start(bt.conn, newpod)
		Expect(err).To(BeNil())

		// Stop the running pods
		_, err = pods.Stop(bt.conn, newpod, nil)
		Expect(err).To(BeNil())
		response, _ = pods.Inspect(bt.conn, newpod)
		Expect(response.State).To(Equal(define.PodStateExited))
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStateStopped))
		}

		// Stop an already stopped pod
		_, err = pods.Stop(bt.conn, newpod, nil)
		Expect(err).To(BeNil())

		_, err = pods.Restart(bt.conn, newpod)
		Expect(err).To(BeNil())
		response, _ = pods.Inspect(bt.conn, newpod)
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
		pruneResponse, err := pods.Prune(bt.conn)
		Expect(err).To(BeNil())
		podSummary, err := pods.List(bt.conn, nil)
		Expect(err).To(BeNil())
		Expect(len(podSummary)).To(Equal(2))

		// Prune only one pod which is in exited state.
		// Start then stop a pod.
		// pod moves to exited state one pod should be pruned now.
		_, err = pods.Start(bt.conn, newpod)
		Expect(err).To(BeNil())
		_, err = pods.Stop(bt.conn, newpod, nil)
		Expect(err).To(BeNil())
		response, err := pods.Inspect(bt.conn, newpod)
		Expect(err).To(BeNil())
		Expect(response.State).To(Equal(define.PodStateExited))
		pruneResponse, err = pods.Prune(bt.conn)
		Expect(err).To(BeNil())
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
		_, err = pods.Start(bt.conn, newpod)
		Expect(err).To(BeNil())
		_, err = pods.Start(bt.conn, newpod2)
		Expect(err).To(BeNil())
		_, err = pods.Stop(bt.conn, newpod, nil)
		Expect(err).To(BeNil())
		response, err = pods.Inspect(bt.conn, newpod)
		Expect(err).To(BeNil())
		Expect(response.State).To(Equal(define.PodStateExited))
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStateStopped))
		}
		_, err = pods.Stop(bt.conn, newpod2, nil)
		Expect(err).To(BeNil())
		response, err = pods.Inspect(bt.conn, newpod2)
		Expect(err).To(BeNil())
		Expect(response.State).To(Equal(define.PodStateExited))
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStateStopped))
		}
		_, err = pods.Prune(bt.conn)
		Expect(err).To(BeNil())
		podSummary, err = pods.List(bt.conn, nil)
		Expect(err).To(BeNil())
		Expect(len(podSummary)).To(Equal(0))
	})

	It("simple create pod", func() {
		ps := specgen.PodSpecGenerator{}
		ps.Name = "foobar"
		_, err := pods.CreatePodFromSpec(bt.conn, &ps)
		Expect(err).To(BeNil())

		exists, err := pods.Exists(bt.conn, "foobar")
		Expect(err).To(BeNil())
		Expect(exists).To(BeTrue())
	})

	// Test validates the pod top bindings
	It("pod top", func() {
		var name string = "podA"

		bt.Podcreate(&name)
		_, err := pods.Start(bt.conn, name)
		Expect(err).To(BeNil())

		// By name
		_, err = pods.Top(bt.conn, name, nil)
		Expect(err).To(BeNil())

		// With descriptors
		output, err := pods.Top(bt.conn, name, []string{"user,pid,hpid"})
		Expect(err).To(BeNil())
		header := strings.Split(output[0], "\t")
		for _, d := range []string{"USER", "PID", "HPID"} {
			Expect(d).To(BeElementOf(header))
		}

		// With bogus ID
		_, err = pods.Top(bt.conn, "IdoNotExist", nil)
		Expect(err).ToNot(BeNil())

		// With bogus descriptors
		_, err = pods.Top(bt.conn, name, []string{"Me,Neither"})
		Expect(err).ToNot(BeNil())
	})
})
