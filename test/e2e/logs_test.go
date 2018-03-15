package integration

import (
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman logs", func() {
	var (
		tempdir    string
		err        error
		podmanTest PodmanTest
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanCreate(tempdir)
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.Cleanup()

	})

	//sudo bin/podman run -it --rm fedora-minimal bash -c 'for a in `seq 5`; do echo hello; done'
	It("podman logs for container", func() {
		podmanTest.RestoreArtifact(fedoraMinimal)
		logc := podmanTest.Podman([]string{"run", "-dt", fedoraMinimal, "bash", "-c", "for i in `seq 5 -1 1`; do echo $i; done"})
		logc.WaitWithDefaultTimeout()
		Expect(logc.ExitCode()).To(Equal(0))
		cid := logc.OutputToString()

		results := podmanTest.Podman([]string{"logs", cid})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		output := results.OutputToStringArray()
		Expect(len(output)).To(Equal(6))
		Expect(output[len(output)-2]).To(Equal("1"))
	})

	It("podman logs tail two lines", func() {
		podmanTest.RestoreArtifact(fedoraMinimal)
		logc := podmanTest.Podman([]string{"run", "-dt", fedoraMinimal, "bash", "-c", "for i in `seq 5 -1 1`; do echo $i; done"})
		logc.WaitWithDefaultTimeout()
		Expect(logc.ExitCode()).To(Equal(0))
		cid := logc.OutputToString()

		results := podmanTest.Podman([]string{"logs", "--tail", "2", cid})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		output := results.OutputToStringArray()
		Expect(len(output)).To(Equal(3))
		Expect(output[len(output)-2]).To(Equal("1"))
	})

	It("podman logs tail 99 lines", func() {
		podmanTest.RestoreArtifact(fedoraMinimal)
		logc := podmanTest.Podman([]string{"run", "-dt", fedoraMinimal, "bash", "-c", "for i in `seq 5 -1 1`; do echo $i; done"})
		logc.WaitWithDefaultTimeout()
		Expect(logc.ExitCode()).To(Equal(0))
		cid := logc.OutputToString()

		results := podmanTest.Podman([]string{"logs", "--tail", "99", cid})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		output := results.OutputToStringArray()
		Expect(len(output)).To(Equal(6))
		Expect(output[len(output)-2]).To(Equal("1"))
	})

	It("podman logs tail 2 lines with timestamps", func() {
		podmanTest.RestoreArtifact(fedoraMinimal)
		logc := podmanTest.Podman([]string{"run", "-dt", fedoraMinimal, "bash", "-c", "for i in `seq 5 -1 1`; do echo $i; done"})
		logc.WaitWithDefaultTimeout()
		Expect(logc.ExitCode()).To(Equal(0))
		cid := logc.OutputToString()

		results := podmanTest.Podman([]string{"logs", "--tail", "2", "-t", cid})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		output := results.OutputToStringArray()
		Expect(len(output)).To(Equal(3))
		Expect(output[len(output)-2]).To(MatchRegexp("\\d+-\\d+-\\d+T\\d+:\\d+:\\d.*1"))
	})

	It("podman logs latest with since time", func() {
		podmanTest.RestoreArtifact(fedoraMinimal)
		logc := podmanTest.Podman([]string{"run", "-dt", fedoraMinimal, "bash", "-c", "for i in `seq 5 -1 1`; do echo $i; done"})
		logc.WaitWithDefaultTimeout()
		Expect(logc.ExitCode()).To(Equal(0))
		cid := logc.OutputToString()

		results := podmanTest.Podman([]string{"logs", "--since", "2017-08-07T10:10:09.056611202-04:00", cid})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		output := results.OutputToStringArray()
		Expect(len(output)).To(Equal(6))
		Expect(output[len(output)-2]).To(Equal("1"))
	})

	It("podman logs latest with since duration", func() {
		podmanTest.RestoreArtifact(fedoraMinimal)
		logc := podmanTest.Podman([]string{"run", "-dt", fedoraMinimal, "bash", "-c", "for i in `seq 5 -1 1`; do echo $i; sleep 2; done"})
		logc.WaitWithDefaultTimeout()
		Expect(logc.ExitCode()).To(Equal(0))
		cid := logc.OutputToString()

		time.Sleep(10 * time.Second)
		results := podmanTest.Podman([]string{"logs", "--since", "5s", "-t", cid})
		results.WaitWithDefaultTimeout()
		Expect(results.ExitCode()).To(Equal(0))
		output := results.OutputToStringArray()
		Expect(len(output)).To(Equal(3))
	})

	It("podman logs follow a running conatiner", func() {
		podmanTest.RestoreArtifact(fedoraMinimal)
		logc := podmanTest.Podman([]string{"run", "-dt", fedoraMinimal, "bash", "-c", "for i in `seq 5 -1 1`; do echo $i; sleep 1; done"})
		logc.WaitWithDefaultTimeout()
		start := time.Now()
		Expect(logc.ExitCode()).To(Equal(0))
		cid := logc.OutputToString()

		results := podmanTest.Podman([]string{"logs", "-f", cid})
		results.WaitWithDefaultTimeout()
		end := time.Now()
		Expect(results.ExitCode()).To(Equal(0))
		output := results.OutputToStringArray()
		Expect(len(output)).To(Equal(6))
		Expect(output[len(output)-2]).To(Equal("1"))
		Expect(end.Sub(start)).To(BeNumerically(">", 5))
	})
})
