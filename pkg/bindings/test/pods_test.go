package test_bindings

import (
	"context"
	"net/http"
	"time"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/bindings"
	"github.com/containers/libpod/pkg/bindings/pods"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman images", func() {
	var (
		bt       *bindingTest
		s        *gexec.Session
		connText context.Context
		newpod   string
		err      error
		trueFlag bool = true
	)

	BeforeEach(func() {
		bt = newBindingTest()
		newpod = "newpod"
		bt.RestoreImagesFromCache()
		bt.Podcreate(&newpod)
		s = bt.startAPIService()
		time.Sleep(1 * time.Second)
		connText, err = bindings.NewConnection(context.Background(), bt.sock)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		s.Kill()
		bt.cleanup()
	})

	It("inspect pod", func() {
		//Inspect an invalid pod name
		_, err := pods.Inspect(connText, "dummyname")
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		//Inspect an valid pod name
		response, err := pods.Inspect(connText, newpod)
		Expect(err).To(BeNil())
		Expect(response.Config.Name).To(Equal(newpod))
	})

	// Test validates the list all api returns
	It("list pod", func() {
		//List all the pods in the current instance
		podSummary, err := pods.List(connText, nil)
		Expect(err).To(BeNil())
		Expect(len(podSummary)).To(Equal(1))
		// Adding an alpine container to the existing pod
		bt.RunTopContainer(nil, &trueFlag, &newpod)
		podSummary, err = pods.List(connText, nil)
		// Verify no errors.
		Expect(err).To(BeNil())
		// Verify number of containers in the pod.
		Expect(len(podSummary[0].Containers)).To(Equal(2))

		// Add multiple pods and verify them by name and size.
		var newpod2 string = "newpod2"
		bt.Podcreate(&newpod2)
		podSummary, err = pods.List(connText, nil)
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
		//filteredPods, err := pods.List(connText, filters)
		//Expect(err).To(BeNil())
		//Expect(len(filteredPods)).To(BeNumerically("==", 1))
	})

	// The test validates if the exists responds
	It("exists pod", func() {
		response, err := pods.Exists(connText, "dummyName")
		Expect(err).To(BeNil())
		Expect(response).To(BeFalse())

		// Should exit with no error and response should be true
		response, err = pods.Exists(connText, "newpod")
		Expect(err).To(BeNil())
		Expect(response).To(BeTrue())
	})

	// This test validates if All running containers within
	// each specified pod are paused and unpaused
	It("pause upause pod", func() {
		// Pause invalid container
		err := pods.Pause(connText, "dummyName")
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Adding an alpine container to the existing pod
		bt.RunTopContainer(nil, &trueFlag, &newpod)
		response, err := pods.Inspect(connText, newpod)
		Expect(err).To(BeNil())

		// Binding needs to be modified to inspect the pod state.
		// Since we dont have a pod state we inspect the states of the containers within the pod.
		// Pause a valid container
		err = pods.Pause(connText, newpod)
		Expect(err).To(BeNil())
		response, err = pods.Inspect(connText, newpod)
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStatePaused))
		}

		// Unpause a valid container
		err = pods.Unpause(connText, newpod)
		Expect(err).To(BeNil())
		response, err = pods.Inspect(connText, newpod)
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStateRunning))
		}
	})

	It("start stop restart pod", func() {
		// Start an invalid pod
		err = pods.Start(connText, "dummyName")
		Expect(err).ToNot(BeNil())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Stop an invalid pod
		err = pods.Stop(connText, "dummyName", nil)
		Expect(err).ToNot(BeNil())
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Restart an invalid pod
		err = pods.Restart(connText, "dummyName")
		Expect(err).ToNot(BeNil())
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// Start a valid pod and inspect status of each container
		err = pods.Start(connText, newpod)
		Expect(err).To(BeNil())

		response, err := pods.Inspect(connText, newpod)
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStateRunning))
		}

		// Start an already running  pod
		err = pods.Start(connText, newpod)
		Expect(err).To(BeNil())

		// Stop the running pods
		err = pods.Stop(connText, newpod, nil)
		Expect(err).To(BeNil())
		response, _ = pods.Inspect(connText, newpod)
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStateStopped))
		}

		// Stop an already stopped pod
		err = pods.Stop(connText, newpod, nil)
		Expect(err).To(BeNil())

		err = pods.Restart(connText, newpod)
		Expect(err).To(BeNil())
		response, _ = pods.Inspect(connText, newpod)
		for _, i := range response.Containers {
			Expect(define.StringToContainerStatus(i.State)).
				To(Equal(define.ContainerStateRunning))
		}
	})

	// Remove all stopped pods and their container to be implemented.
	It("prune pod", func() {
	})
})
