package test_bindings

import (
	"net/http"
	"time"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/bindings"
	"github.com/containers/libpod/pkg/bindings/pods"
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
		Expect(response.Config.Name).To(Equal(newpod))
	})

	// Test validates the list all api returns
	It("list pod", func() {
		//List all the pods in the current instance
		podSummary, err := pods.List(bt.conn, nil)
		Expect(err).To(BeNil())
		Expect(len(podSummary)).To(Equal(1))
		// Adding an alpine container to the existing pod
		_, err = bt.RunTopContainer(nil, &bindings.PTrue, &newpod)
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
			names = append(names, i.Config.Name)
		}
		Expect(StringInSlice(newpod, names)).To(BeTrue())
		Expect(StringInSlice("newpod2", names)).To(BeTrue())

		// TODO not working  Because: code to list based on filter
		// "not yet implemented",
		// Validate list pod with filters
		//filters := make(map[string][]string)
		//filters["name"] = []string{newpod}
		//filteredPods, err := pods.List(bt.conn, filters)
		//Expect(err).To(BeNil())
		//Expect(len(filteredPods)).To(BeNumerically("==", 1))
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
		err := pods.Pause(bt.conn, "dummyName")
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Adding an alpine container to the existing pod
		_, err = bt.RunTopContainer(nil, &bindings.PTrue, &newpod)
		Expect(err).To(BeNil())

		// Binding needs to be modified to inspect the pod state.
		// Since we don't have a pod state we inspect the states of the containers within the pod.
		// Pause a valid container
		err = pods.Pause(bt.conn, newpod)
		Expect(err).To(BeNil())
		response, err := pods.Inspect(bt.conn, newpod)
		Expect(response.State.Status).To(Equal(define.PodStatePaused))
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStatePaused))
		}

		// Unpause a valid container
		err = pods.Unpause(bt.conn, newpod)
		Expect(err).To(BeNil())
		response, err = pods.Inspect(bt.conn, newpod)
		Expect(response.State.Status).To(Equal(define.PodStateRunning))
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStateRunning))
		}
	})

	It("start stop restart pod", func() {
		// Start an invalid pod
		err = pods.Start(bt.conn, "dummyName")
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Stop an invalid pod
		err = pods.Stop(bt.conn, "dummyName", nil)
		Expect(err).ToNot(BeNil())
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Restart an invalid pod
		err = pods.Restart(bt.conn, "dummyName")
		Expect(err).ToNot(BeNil())
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Start a valid pod and inspect status of each container
		err = pods.Start(bt.conn, newpod)
		Expect(err).To(BeNil())

		response, err := pods.Inspect(bt.conn, newpod)
		Expect(response.State.Status).To(Equal(define.PodStateRunning))
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStateRunning))
		}

		// Start an already running  pod
		err = pods.Start(bt.conn, newpod)
		Expect(err).To(BeNil())

		// Stop the running pods
		err = pods.Stop(bt.conn, newpod, nil)
		Expect(err).To(BeNil())
		response, _ = pods.Inspect(bt.conn, newpod)
		Expect(response.State.Status).To(Equal(define.PodStateExited))
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStateStopped))
		}

		// Stop an already stopped pod
		err = pods.Stop(bt.conn, newpod, nil)
		Expect(err).To(BeNil())

		err = pods.Restart(bt.conn, newpod)
		Expect(err).To(BeNil())
		response, _ = pods.Inspect(bt.conn, newpod)
		Expect(response.State.Status).To(Equal(define.PodStateRunning))
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
		err = pods.Prune(bt.conn)
		Expect(err).To(BeNil())
		podSummary, err := pods.List(bt.conn, nil)
		Expect(err).To(BeNil())
		Expect(len(podSummary)).To(Equal(2))

		// Prune only one pod which is in exited state.
		// Start then stop a pod.
		// pod moves to exited state one pod should be pruned now.
		err = pods.Start(bt.conn, newpod)
		Expect(err).To(BeNil())
		err = pods.Stop(bt.conn, newpod, nil)
		Expect(err).To(BeNil())
		response, err := pods.Inspect(bt.conn, newpod)
		Expect(response.State.Status).To(Equal(define.PodStateExited))
		err = pods.Prune(bt.conn)
		Expect(err).To(BeNil())
		podSummary, err = pods.List(bt.conn, nil)
		Expect(err).To(BeNil())
		Expect(len(podSummary)).To(Equal(1))

		// Test prune all pods in exited state.
		bt.Podcreate(&newpod)
		err = pods.Start(bt.conn, newpod)
		Expect(err).To(BeNil())
		err = pods.Start(bt.conn, newpod2)
		Expect(err).To(BeNil())
		err = pods.Stop(bt.conn, newpod, nil)
		Expect(err).To(BeNil())
		response, err = pods.Inspect(bt.conn, newpod)
		Expect(response.State.Status).To(Equal(define.PodStateExited))
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStateStopped))
		}
		err = pods.Stop(bt.conn, newpod2, nil)
		Expect(err).To(BeNil())
		response, err = pods.Inspect(bt.conn, newpod2)
		Expect(response.State.Status).To(Equal(define.PodStateExited))
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStateStopped))
		}
		err = pods.Prune(bt.conn)
		Expect(err).To(BeNil())
		podSummary, err = pods.List(bt.conn, nil)
		Expect(err).To(BeNil())
		Expect(len(podSummary)).To(Equal(0))
	})
})
