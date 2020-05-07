package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	. "github.com/containers/libpod/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman events", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		SkipIfRootlessV2()
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.SeedImages()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))

	})

	// For most, all, of these tests we do not "live" test following a log because it may make a fragile test
	// system more complex.  Instead we run the "events" and then verify that the events are processed correctly.
	// Perhaps a future version of this test would put events in a go func and send output back over a channel
	// while events occur.
	It("podman events", func() {
		Skip("need to verify images have correct packages for journald")
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		result := podmanTest.Podman([]string{"events", "--stream=false"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(BeZero())
	})

	It("podman events with an event filter", func() {
		Skip("need to verify images have correct packages for journald")
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		result := podmanTest.Podman([]string{"events", "--stream=false", "--filter", "event=start"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray()) >= 1)
	})

	It("podman events with an event filter and container=cid", func() {
		Skip("need to verify images have correct packages for journald")
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		_, ec2, cid2 := podmanTest.RunLsContainer("")
		Expect(ec2).To(Equal(0))
		result := podmanTest.Podman([]string{"events", "--stream=false", "--filter", "event=start", "--filter", fmt.Sprintf("container=%s", cid)})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).To(Equal(1))
		Expect(!strings.Contains(result.OutputToString(), cid2))
	})

	It("podman events with a type and filter container=id", func() {
		Skip("need to verify images have correct packages for journald")
		_, ec, cid := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		result := podmanTest.Podman([]string{"events", "--stream=false", "--filter", "type=pod", "--filter", fmt.Sprintf("container=%s", cid)})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).To(Equal(0))
	})

	It("podman events with a type", func() {
		Skip("need to verify images have correct packages for journald")
		setup := podmanTest.Podman([]string{"run", "-dt", "--pod", "new:foobarpod", ALPINE, "top"})
		setup.WaitWithDefaultTimeout()
		stop := podmanTest.Podman([]string{"pod", "stop", "foobarpod"})
		stop.WaitWithDefaultTimeout()
		Expect(stop.ExitCode()).To(Equal(0))
		Expect(setup.ExitCode()).To(Equal(0))
		result := podmanTest.Podman([]string{"events", "--stream=false", "--filter", "type=pod", "--filter", "pod=foobarpod"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		fmt.Println(result.OutputToStringArray())
		Expect(len(result.OutputToStringArray()) >= 2)
	})

	It("podman events --since", func() {
		Skip("need to verify images have correct packages for journald")
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		result := podmanTest.Podman([]string{"events", "--stream=false", "--since", "1m"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(BeZero())
	})

	It("podman events --until", func() {
		Skip("need to verify images have correct packages for journald")
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		test := podmanTest.Podman([]string{"events", "--help"})
		test.WaitWithDefaultTimeout()
		fmt.Println(test.OutputToStringArray())
		result := podmanTest.Podman([]string{"events", "--stream=false", "--since", "1h"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(BeZero())
	})

	It("podman events format", func() {
		info := GetHostDistributionInfo()
		if info.Distribution != "fedora" {
			Skip("need to verify images have correct packages for journald")
		}
		_, ec, _ := podmanTest.RunLsContainer("")
		Expect(ec).To(Equal(0))
		test := podmanTest.Podman([]string{"events", "--stream=false", "--format", "json"})
		test.WaitWithDefaultTimeout()
		fmt.Println(test.OutputToStringArray())
		jsonArr := test.OutputToStringArray()
		Expect(len(jsonArr)).To(Not(BeZero()))
		eventsMap := make(map[string]string)
		err := json.Unmarshal([]byte(jsonArr[0]), &eventsMap)
		if err != nil {
			os.Exit(1)
		}
		_, exist := eventsMap["Status"]
		Expect(exist).To(BeTrue())
		Expect(test.ExitCode()).To(BeZero())
	})
})
