package utils_test

import (
	"os"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PodmanTest test", func() {
	var podmanTest *FakePodmanTest

	BeforeEach(func() {
		podmanTest = FakePodmanTestCreate()
	})

	AfterEach(func() {
		FakeOutputs = make(map[string][]string)
	})

	It("Test PodmanAsUserBase", func() {
		FakeOutputs["check"] = []string{"check"}
		os.Setenv("HOOK_OPTION", "hook_option")
		env := os.Environ()
		session := podmanTest.PodmanAsUserBase([]string{"check"}, 1000, 1000, "", env, true, false, nil, nil)
		os.Unsetenv("HOOK_OPTION")
		session.WaitWithDefaultTimeout()
		Expect(session.Command.Process).ShouldNot(BeNil())
	})

	It("Test NumberOfContainersRunning", func() {
		FakeOutputs["ps -q"] = []string{"one", "two"}
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
	})

	It("Test NumberOfContainers", func() {
		FakeOutputs["ps -aq"] = []string{"one", "two"}
		Expect(podmanTest.NumberOfContainers()).To(Equal(2))
	})

	It("Test NumberOfPods", func() {
		FakeOutputs["pod ps -q"] = []string{"one", "two"}
		Expect(podmanTest.NumberOfPods()).To(Equal(2))
	})

	It("Test WaitForContainer", func() {
		FakeOutputs["ps -q"] = []string{"one", "two"}
		Expect(WaitForContainer(podmanTest)).To(BeTrue())

		FakeOutputs["ps -q"] = []string{"one"}
		Expect(WaitForContainer(podmanTest)).To(BeTrue())

		FakeOutputs["ps -q"] = []string{""}
		Expect(WaitForContainer(podmanTest)).To(Not(BeTrue()))
	})

	It("Test GetContainerStatus", func() {
		FakeOutputs["ps --all --format={{.Status}}"] = []string{"Need func update"}
		Expect(podmanTest.GetContainerStatus()).To(Equal("Need func update"))
	})

	It("Test WaitContainerReady", func() {
		FakeOutputs["logs testimage"] = []string{""}
		Expect(WaitContainerReady(podmanTest, "testimage", "ready", 2, 1)).To(Not(BeTrue()))

		FakeOutputs["logs testimage"] = []string{"I am ready"}
		Expect(WaitContainerReady(podmanTest, "testimage", "am ready", 2, 1)).To(BeTrue())

		FakeOutputs["logs testimage"] = []string{"I am ready"}
		Expect(WaitContainerReady(podmanTest, "testimage", "", 2, 1)).To(BeTrue())
	})

})
