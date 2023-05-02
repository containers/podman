package bindings_test

import (
	"net/http"
	"strings"
	"time"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/bindings/containers"
	"github.com/containers/podman/v4/pkg/domain/entities/reports"
	"github.com/containers/podman/v4/pkg/specgen"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman containers ", func() {
	var (
		bt  *bindingTest
		s   *gexec.Session
		err error
	)

	BeforeEach(func() {
		bt = newBindingTest()
		bt.RestoreImagesFromCache()
		s = bt.startAPIService()
		time.Sleep(1 * time.Second)
		err := bt.NewConnection()
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		s.Kill()
		bt.cleanup()
	})

	It("podman pause a bogus container", func() {
		// Pausing bogus container should return 404
		err = containers.Pause(bt.conn, "foobar", nil)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))
	})

	It("podman unpause a bogus container", func() {
		// Unpausing bogus container should return 404
		err = containers.Unpause(bt.conn, "foobar", nil)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))
	})

	It("podman pause a running container by name", func() {
		// Pausing by name should work
		var name = "top"
		_, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Pause(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())

		// Ensure container is paused
		data, err := containers.Inspect(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(data.State.Status).To(Equal("paused"))
	})

	It("podman pause a running container by id", func() {
		// Pausing by id should work
		var name = "top"
		cid, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Pause(bt.conn, cid, nil)
		Expect(err).ToNot(HaveOccurred())

		// Ensure container is paused
		data, err := containers.Inspect(bt.conn, cid, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(data.State.Status).To(Equal("paused"))
	})

	It("podman unpause a running container by name", func() {
		// Unpausing by name should work
		var name = "top"
		_, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Pause(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Unpause(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())

		// Ensure container is unpaused
		data, err := containers.Inspect(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(data.State.Status).To(Equal("running"))
	})

	It("podman unpause a running container by ID", func() {
		// Unpausing by ID should work
		var name = "top"
		_, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		// Pause by name
		err = containers.Pause(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred(), "error from containers.Pause()")
		// paused := "paused"
		// _, err = containers.Wait(bt.conn, cid, &paused)
		// Expect(err).To(BeNil())
		err = containers.Unpause(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())

		// Ensure container is unpaused
		data, err := containers.Inspect(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(data.State.Status).To(Equal("running"))
	})

	It("podman pause a paused container by name", func() {
		// Pausing a paused container by name should fail
		var name = "top"
		_, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Pause(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Pause(bt.conn, name, nil)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("podman pause a paused container by id", func() {
		// Pausing a paused container by id should fail
		var name = "top"
		cid, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Pause(bt.conn, cid, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Pause(bt.conn, cid, nil)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("podman pause a stopped container by name", func() {
		// Pausing a stopped container by name should fail
		var name = "top"
		_, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Stop(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Pause(bt.conn, name, nil)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("podman pause a stopped container by id", func() {
		// Pausing a stopped container by id should fail
		var name = "top"
		cid, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Stop(bt.conn, cid, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Pause(bt.conn, cid, nil)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("podman remove a paused container by id without force", func() {
		// Removing a paused container without force should fail
		var name = "top"
		cid, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Pause(bt.conn, cid, nil)
		Expect(err).ToNot(HaveOccurred())
		_, err = containers.Remove(bt.conn, cid, nil)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("podman remove a paused container by id with force", func() {
		// Removing a paused container with force should work
		var name = "top"
		cid, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Pause(bt.conn, cid, nil)
		Expect(err).ToNot(HaveOccurred())
		rmResponse, err := containers.Remove(bt.conn, cid, new(containers.RemoveOptions).WithForce(true))
		Expect(err).ToNot(HaveOccurred())
		Expect(reports.RmReportsErrs(rmResponse)).To(BeEmpty())
		Expect(reports.RmReportsIds(rmResponse)).To(HaveLen(1))
	})

	It("podman stop a paused container by name", func() {
		// Stopping a paused container by name should fail
		var name = "top"
		_, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Pause(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Stop(bt.conn, name, nil)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("podman stop a paused container by id", func() {
		// Stopping a paused container by id should fail
		var name = "top"
		cid, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Pause(bt.conn, cid, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Stop(bt.conn, cid, nil)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("podman stop a running container by name", func() {
		// Stopping a running container by name should work
		var name = "top"
		_, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Stop(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())

		// Ensure container is stopped
		data, err := containers.Inspect(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(isStopped(data.State.Status)).To(BeTrue())
	})

	It("podman stop a running container by ID", func() {
		// Stopping a running container by ID should work
		var name = "top"
		cid, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Stop(bt.conn, cid, nil)
		Expect(err).ToNot(HaveOccurred())

		// Ensure container is stopped
		data, err := containers.Inspect(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(isStopped(data.State.Status)).To(BeTrue())
	})

	It("podman wait no condition", func() {
		var (
			name           = "top"
			exitCode int32 = -1
		)
		_, err := containers.Wait(bt.conn, "foobar", nil)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		errChan := make(chan error)
		_, err = bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		go func() {
			defer GinkgoRecover()
			exitCode, err = containers.Wait(bt.conn, name, nil)
			errChan <- err
			close(errChan)
		}()
		err = containers.Stop(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())
		wait := <-errChan
		Expect(wait).ToNot(HaveOccurred())
		Expect(exitCode).To(BeNumerically("==", 143))
	})

	It("podman wait to pause|unpause condition", func() {
		var (
			name           = "top"
			exitCode int32 = -1
			pause          = define.ContainerStatePaused
			running        = define.ContainerStateRunning
		)
		errChan := make(chan error)
		_, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		go func() {
			defer GinkgoRecover()
			exitCode, err = containers.Wait(bt.conn, name, new(containers.WaitOptions).WithCondition([]define.ContainerStatus{pause}))
			errChan <- err
			close(errChan)
		}()
		err = containers.Pause(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())
		wait := <-errChan
		Expect(wait).ToNot(HaveOccurred())
		Expect(exitCode).To(BeNumerically("==", -1))

		unpauseErrChan := make(chan error)
		go func() {
			defer GinkgoRecover()

			_, waitErr := containers.Wait(bt.conn, name, new(containers.WaitOptions).WithCondition([]define.ContainerStatus{running}))
			unpauseErrChan <- waitErr
			close(unpauseErrChan)
		}()
		err = containers.Unpause(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())
		unPausewait := <-unpauseErrChan
		Expect(unPausewait).ToNot(HaveOccurred())
		Expect(exitCode).To(BeNumerically("==", -1))
	})

	It("run  healthcheck", func() {
		bt.runPodman([]string{"run", "-d", "--name", "hc", "--health-interval", "disable", "--health-retries", "2", "--health-cmd", "ls / || exit 1", alpine.name, "top"})

		// bogus name should result in 404
		_, err := containers.RunHealthCheck(bt.conn, "foobar", nil)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))

		// a container that has no healthcheck should be a 409
		var name = "top"
		_, err = bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		_, err = containers.RunHealthCheck(bt.conn, name, nil)
		Expect(err).To(HaveOccurred())
		code, _ = bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusConflict))

		// TODO for the life of me, i cannot get this to work. maybe another set
		// of eyes will
		// successful healthcheck
		// status := define.HealthCheckHealthy
		// for i:=0; i < 10; i++ {
		//	result, err := containers.RunHealthCheck(connText, "hc")
		//	Expect(err).To(BeNil())
		//	if result.Status != define.HealthCheckHealthy {
		//		fmt.Println("Healthcheck container still starting, retrying in 1 second")
		//		time.Sleep(1 * time.Second)
		//		continue
		//	}
		//	status = result.Status
		//	break
		// }
		// Expect(status).To(Equal(define.HealthCheckHealthy))

		// TODO enable this when wait is working
		// healthcheck on a stopped container should be a 409
		// err = containers.Stop(connText, "hc", nil)
		// Expect(err).To(BeNil())
		// _, err = containers.Wait(connText, "hc")
		// Expect(err).To(BeNil())
		// _, err = containers.RunHealthCheck(connText, "hc")
		// code, _ = bindings.CheckResponseCode(err)
		// Expect(code).To(BeNumerically("==", http.StatusConflict))
	})

	It("logging", func() {
		stdoutChan := make(chan string, 10)
		s := specgen.NewSpecGenerator(alpine.name, false)
		s.Terminal = true
		s.Command = []string{"date", "-R"}
		r, err := containers.CreateWithSpec(bt.conn, s, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Start(bt.conn, r.ID, nil)
		Expect(err).ToNot(HaveOccurred())

		_, err = containers.Wait(bt.conn, r.ID, nil)
		Expect(err).ToNot(HaveOccurred())

		opts := new(containers.LogOptions).WithStdout(true).WithFollow(true)
		go func() {
			defer GinkgoRecover()
			err := containers.Logs(bt.conn, r.ID, opts, stdoutChan, nil)
			close(stdoutChan)
			Expect(err).ShouldNot(HaveOccurred())
		}()
		o := <-stdoutChan
		o = strings.TrimSpace(o)
		_, err = time.Parse(time.RFC1123Z, o)
		Expect(err).ShouldNot(HaveOccurred())
	})

	It("podman top", func() {
		var name = "top"
		cid, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())

		// By name
		_, err = containers.Top(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())

		// By id
		_, err = containers.Top(bt.conn, cid, nil)
		Expect(err).ToNot(HaveOccurred())

		// With descriptors
		output, err := containers.Top(bt.conn, cid, new(containers.TopOptions).WithDescriptors([]string{"user", "pid", "hpid"}))
		Expect(err).ToNot(HaveOccurred())
		header := strings.Split(output[0], "\t")
		for _, d := range []string{"USER", "PID", "HPID"} {
			Expect(d).To(BeElementOf(header))
		}

		// With bogus ID
		_, err = containers.Top(bt.conn, "IdoNotExist", nil)
		Expect(err).To(HaveOccurred())

		// With bogus descriptors
		_, err = containers.Top(bt.conn, cid, new(containers.TopOptions).WithDescriptors([]string{"Me,Neither"}))
		Expect(err).ToNot(HaveOccurred())
	})

	It("podman bogus container does not exist in local storage", func() {
		// Bogus container existence check should fail
		containerExists, err := containers.Exists(bt.conn, "foobar", nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(containerExists).To(BeFalse())
	})

	It("podman container exists in local storage by name", func() {
		// Container existence check by name should work
		var name = "top"
		_, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		containerExists, err := containers.Exists(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(containerExists).To(BeTrue())
	})

	It("podman container exists in local storage by ID", func() {
		// Container existence check by ID should work
		var name = "top"
		cid, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		containerExists, err := containers.Exists(bt.conn, cid, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(containerExists).To(BeTrue())
	})

	It("podman container exists in local storage by short ID", func() {
		// Container existence check by short ID should work
		var name = "top"
		cid, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		containerExists, err := containers.Exists(bt.conn, cid[0:12], nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(containerExists).To(BeTrue())
	})

	It("podman kill bogus container", func() {
		// Killing bogus container should return 404
		err := containers.Kill(bt.conn, "foobar", new(containers.KillOptions).WithSignal("SIGTERM"))
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))
	})

	It("podman kill a running container by name with SIGINT", func() {
		// Killing a running container should work
		var name = "top"
		_, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Kill(bt.conn, name, new(containers.KillOptions).WithSignal("SIGINT"))
		Expect(err).ToNot(HaveOccurred())
		_, err = containers.Exists(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())
	})

	It("podman kill a running container by ID with SIGTERM", func() {
		// Killing a running container by ID should work
		var name = "top"
		cid, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Kill(bt.conn, cid, new(containers.KillOptions).WithSignal("SIGTERM"))
		Expect(err).ToNot(HaveOccurred())
		_, err = containers.Exists(bt.conn, cid, nil)
		Expect(err).ToNot(HaveOccurred())
	})

	It("podman kill a running container by ID with SIGKILL", func() {
		// Killing a running container by ID with TERM should work
		var name = "top"
		cid, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Kill(bt.conn, cid, new(containers.KillOptions).WithSignal("SIGKILL"))
		Expect(err).ToNot(HaveOccurred())
	})

	It("podman kill a running container by bogus signal", func() {
		// Killing a running container by bogus signal should fail
		var name = "top"
		cid, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Kill(bt.conn, cid, new(containers.KillOptions).WithSignal("foobar"))
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("podman kill latest container with SIGTERM", func() {
		// Killing latest container should work
		var name1 = "first"
		var name2 = "second"
		_, err := bt.RunTopContainer(&name1, nil)
		Expect(err).ToNot(HaveOccurred())
		_, err = bt.RunTopContainer(&name2, nil)
		Expect(err).ToNot(HaveOccurred())
		containerLatestList, err := containers.List(bt.conn, new(containers.ListOptions).WithLast(1))
		Expect(err).ToNot(HaveOccurred())
		err = containers.Kill(bt.conn, containerLatestList[0].Names[0], new(containers.KillOptions).WithSignal("SIGTERM"))
		Expect(err).ToNot(HaveOccurred())
	})

	It("container init on a bogus container", func() {
		err := containers.ContainerInit(bt.conn, "doesnotexist", nil)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))
	})

	It("container init", func() {
		s := specgen.NewSpecGenerator(alpine.name, false)
		ctr, err := containers.CreateWithSpec(bt.conn, s, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.ContainerInit(bt.conn, ctr.ID, nil)
		Expect(err).ToNot(HaveOccurred())
		//	trying to init again should be an error
		err = containers.ContainerInit(bt.conn, ctr.ID, nil)
		Expect(err).To(HaveOccurred())
	})

	It("podman prune stopped containers", func() {
		// Start and stop a container to enter in exited state.
		var name = "top"
		_, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Stop(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())

		// Prune container should return no errors and one pruned container ID.
		pruneResponse, err := containers.Prune(bt.conn, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(reports.PruneReportsErrs(pruneResponse)).To(BeEmpty())
		Expect(reports.PruneReportsIds(pruneResponse)).To(HaveLen(1))
	})

	It("podman prune stopped containers with filters", func() {
		// Start and stop a container to enter in exited state.
		var name = "top"
		_, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Stop(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())

		// Invalid filter keys should return error.
		filtersIncorrect := map[string][]string{
			"status": {"dummy"},
		}
		_, err = containers.Prune(bt.conn, new(containers.PruneOptions).WithFilters(filtersIncorrect))
		Expect(err).To(HaveOccurred())

		// List filter params should not work with prune.
		filtersIncorrect = map[string][]string{
			"name": {"top"},
		}
		_, err = containers.Prune(bt.conn, new(containers.PruneOptions).WithFilters(filtersIncorrect))
		Expect(err).To(HaveOccurred())

		// Mismatched filter params no container should be pruned.
		filtersIncorrect = map[string][]string{
			"label": {"xyz"},
		}
		pruneResponse, err := containers.Prune(bt.conn, new(containers.PruneOptions).WithFilters(filtersIncorrect))
		Expect(err).ToNot(HaveOccurred())
		Expect(reports.PruneReportsIds(pruneResponse)).To(BeEmpty())
		Expect(reports.PruneReportsErrs(pruneResponse)).To(BeEmpty())

		// Valid filter params container should be pruned now.
		filters := map[string][]string{
			"until": {"5000000000"}, // Friday, June 11, 2128
		}
		pruneResponse, err = containers.Prune(bt.conn, new(containers.PruneOptions).WithFilters(filters))
		Expect(err).ToNot(HaveOccurred())
		Expect(reports.PruneReportsErrs(pruneResponse)).To(BeEmpty())
		Expect(reports.PruneReportsIds(pruneResponse)).To(HaveLen(1))
	})

	It("podman list containers with until filter", func() {
		var name = "top"
		_, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())

		filters := map[string][]string{
			"until": {"5000000000"}, // Friday, June 11, 2128
		}
		c, err := containers.List(bt.conn, new(containers.ListOptions).WithFilters(filters).WithAll(true))
		Expect(err).ToNot(HaveOccurred())
		Expect(c).To(HaveLen(1))

		filters = map[string][]string{
			"until": {"500000"}, // Tuesday, January 6, 1970
		}
		c, err = containers.List(bt.conn, new(containers.ListOptions).WithFilters(filters).WithAll(true))
		Expect(err).ToNot(HaveOccurred())
		Expect(c).To(BeEmpty())
	})

	It("podman prune running containers", func() {
		// Start the container.
		var name = "top"
		_, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())

		// Check if the container is running.
		data, err := containers.Inspect(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(data.State.Status).To(Equal("running"))

		// Prune. Should return no error no prune response ID.
		pruneResponse, err := containers.Prune(bt.conn, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(pruneResponse).To(BeEmpty())
	})

	It("podman inspect bogus container", func() {
		_, err := containers.Inspect(bt.conn, "foobar", nil)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))
	})

	It("podman inspect running container", func() {
		var name = "top"
		_, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		// Inspecting running container should succeed
		_, err = containers.Inspect(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())
	})

	It("podman inspect stopped container", func() {
		var name = "top"
		_, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Stop(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())
		// Inspecting stopped container should succeed
		_, err = containers.Inspect(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())
	})

	It("podman inspect running container with size", func() {
		var name = "top"
		_, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		_, err = containers.Inspect(bt.conn, name, new(containers.InspectOptions).WithSize(true))
		Expect(err).ToNot(HaveOccurred())
	})

	It("podman inspect stopped container with size", func() {
		var name = "top"
		_, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		err = containers.Stop(bt.conn, name, nil)
		Expect(err).ToNot(HaveOccurred())
		// Inspecting stopped container with size should succeed
		_, err = containers.Inspect(bt.conn, name, new(containers.InspectOptions).WithSize(true))
		Expect(err).ToNot(HaveOccurred())
	})

	It("podman remove bogus container", func() {
		_, err := containers.Remove(bt.conn, "foobar", nil)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusNotFound))
	})

	It("podman remove running container by name", func() {
		var name = "top"
		_, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		// Removing running container should fail
		_, err = containers.Remove(bt.conn, name, nil)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("podman remove running container by ID", func() {
		var name = "top"
		cid, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		// Removing running container should fail
		_, err = containers.Remove(bt.conn, cid, nil)
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("podman forcibly remove running container by name", func() {
		var name = "top"
		_, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		// Removing running container should succeed
		rmResponse, err := containers.Remove(bt.conn, name, new(containers.RemoveOptions).WithForce(true))
		Expect(err).ToNot(HaveOccurred())
		Expect(reports.RmReportsErrs(rmResponse)).To(BeEmpty())
		Expect(reports.RmReportsIds(rmResponse)).To(HaveLen(1))
	})

	It("podman forcibly remove running container by ID", func() {
		var name = "top"
		cid, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		// Forcibly Removing running container should succeed
		rmResponse, err := containers.Remove(bt.conn, cid, new(containers.RemoveOptions).WithForce(true))
		Expect(err).ToNot(HaveOccurred())
		Expect(reports.RmReportsErrs(rmResponse)).To(BeEmpty())
		Expect(reports.RmReportsIds(rmResponse)).To(HaveLen(1))
	})

	It("podman remove running container and volume by name", func() {
		var name = "top"
		_, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		// Removing running container should fail
		_, err = containers.Remove(bt.conn, name, new(containers.RemoveOptions).WithVolumes(true))
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("podman remove running container and volume by ID", func() {
		var name = "top"
		cid, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		// Removing running container should fail
		_, err = containers.Remove(bt.conn, cid, new(containers.RemoveOptions).WithVolumes(true))
		Expect(err).To(HaveOccurred())
		code, _ := bindings.CheckResponseCode(err)
		Expect(code).To(BeNumerically("==", http.StatusInternalServerError))
	})

	It("podman forcibly remove running container and volume by name", func() {
		var name = "top"
		_, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		// Forcibly Removing running container should succeed
		rmResponse, err := containers.Remove(bt.conn, name, new(containers.RemoveOptions).WithVolumes(true).WithForce(true))
		Expect(err).ToNot(HaveOccurred())
		Expect(reports.RmReportsErrs(rmResponse)).To(BeEmpty())
		Expect(reports.RmReportsIds(rmResponse)).To(HaveLen(1))
	})

	It("podman forcibly remove running container and volume by ID", func() {
		var name = "top"
		cid, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		// Removing running container should fail
		rmResponse, err := containers.Remove(bt.conn, cid, new(containers.RemoveOptions).WithForce(true).WithVolumes(true))
		Expect(err).ToNot(HaveOccurred())
		Expect(reports.RmReportsErrs(rmResponse)).To(BeEmpty())
		Expect(reports.RmReportsIds(rmResponse)).To(HaveLen(1))
	})

	It("List containers with filters", func() {
		var name = "top"
		var name2 = "top2"
		cid, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())
		_, err = bt.RunTopContainer(&name2, nil)
		Expect(err).ToNot(HaveOccurred())
		s := specgen.NewSpecGenerator(alpine.name, false)
		s.Terminal = true
		s.Command = []string{"date", "-R"}
		_, err = containers.CreateWithSpec(bt.conn, s, nil)
		Expect(err).ToNot(HaveOccurred())
		// Validate list container with id filter
		filters := make(map[string][]string)
		filters["id"] = []string{cid}
		c, err := containers.List(bt.conn, new(containers.ListOptions).WithFilters(filters).WithAll(true))
		Expect(err).ToNot(HaveOccurred())
		Expect(c).To(HaveLen(1))
	})

	It("List containers always includes pod information", func() {
		podName := "testpod"
		ctrName := "testctr"
		bt.Podcreate(&podName)
		_, err := bt.RunTopContainer(&ctrName, &podName)
		Expect(err).ToNot(HaveOccurred())

		lastNum := 1

		c, err := containers.List(bt.conn, new(containers.ListOptions).WithAll(true).WithLast(lastNum))
		Expect(err).ToNot(HaveOccurred())
		Expect(c).To(HaveLen(1))
		Expect(c[0].PodName).To(Equal(podName))
	})
})
