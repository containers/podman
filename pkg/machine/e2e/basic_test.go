package e2e_test

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/containers/podman/v5/pkg/machine/define"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("run basic podman commands", func() {

	It("Basic ops", func() {
		// golangci-lint has trouble with actually skipping tests marked Skip
		// so skip it on cirrus envs and where CIRRUS_CI isn't set.
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		bm := basicMachine{}
		imgs, err := mb.setCmd(bm.withPodmanCommand([]string{"images", "-q"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(imgs).To(Exit(0))
		Expect(imgs.outputToStringSlice()).To(BeEmpty())

		newImgs, err := mb.setCmd(bm.withPodmanCommand([]string{"pull", "quay.io/libpod/alpine_nginx"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(newImgs).To(Exit(0))
		Expect(newImgs.outputToStringSlice()).To(HaveLen(1))

		runAlp, err := mb.setCmd(bm.withPodmanCommand([]string{"run", "quay.io/libpod/alpine_nginx", "cat", "/etc/os-release"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(runAlp).To(Exit(0))
		Expect(runAlp.outputToString()).To(ContainSubstring("Alpine Linux"))

		contextDir := GinkgoT().TempDir()
		cfile := filepath.Join(contextDir, "Containerfile")
		err = os.WriteFile(cfile, []byte("FROM quay.io/libpod/alpine_nginx\nRUN ip addr\n"), 0o644)
		Expect(err).ToNot(HaveOccurred())

		build, err := mb.setCmd(bm.withPodmanCommand([]string{"build", contextDir})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(build).To(Exit(0))
		Expect(build.outputToString()).To(ContainSubstring("COMMIT"))

		rmCon, err := mb.setCmd(bm.withPodmanCommand([]string{"rm", "-a"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(rmCon).To(Exit(0))
	})

	It("Volume ops", func() {
		tDir, err := filepath.Abs(GinkgoT().TempDir())
		Expect(err).ToNot(HaveOccurred())
		roFile := filepath.Join(tDir, "attr-test-file")

		// Create the file as ready-only, since some platforms disallow selinux attr writes
		// The subsequent Z mount should still succeed in spite of that
		rf, err := os.OpenFile(roFile, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0o444)
		Expect(err).ToNot(HaveOccurred())
		rf.Close()

		name := randomString()
		i := new(initMachine).withImage(mb.imagePath).withNow()

		// All other platforms have an implicit mount for the temp area
		if isVmtype(define.QemuVirt) {
			i.withVolume(tDir)
		}
		session, err := mb.setName(name).setCmd(i).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		bm := basicMachine{}
		// Test relabel works on all platforms
		runAlp, err := mb.setCmd(bm.withPodmanCommand([]string{"run", "-v", tDir + ":/test:Z", "quay.io/libpod/alpine_nginx", "ls", "/test/attr-test-file"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(runAlp).To(Exit(0))

		// Test build with --volume option
		cf := filepath.Join(tDir, "Containerfile")
		err = os.WriteFile(cf, []byte("FROM quay.io/libpod/alpine_nginx\nRUN ls /test/attr-test-file\n"), 0o644)
		Expect(err).ToNot(HaveOccurred())
		build, err := mb.setCmd(bm.withPodmanCommand([]string{"build", "-t", name, "-v", tDir + ":/test", tDir})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(build).To(Exit(0))
	})

	It("Volume should be virtiofs", func() {
		// In theory this could run on MacOS too, but we know virtiofs works for that now,
		// this is just testing linux
		skipIfNotVmtype(define.QemuVirt, "This is just adding coverage for virtiofs on linux")

		tDir, err := filepath.Abs(GinkgoT().TempDir())
		Expect(err).ToNot(HaveOccurred())

		err = os.WriteFile(filepath.Join(tDir, "testfile"), []byte("some test contents"), 0o644)
		Expect(err).ToNot(HaveOccurred())

		name := randomString()
		i := new(initMachine).withImage(mb.imagePath).withNow()

		// Ensure that this is a volume, it may not be automatically on qemu
		i.withVolume(tDir)
		session, err := mb.setName(name).setCmd(i).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		ssh := new(sshMachine).withSSHCommand([]string{"findmnt", "-no", "FSTYPE", tDir})
		findmnt, err := mb.setName(name).setCmd(ssh).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(findmnt).To(Exit(0))
		Expect(findmnt.outputToString()).To(ContainSubstring("virtiofs"))
	})

	It("Volume should be disabled by command line", func() {
		skipIfWSL("Requires standard volume handling")
		skipIfVmtype(define.AppleHvVirt, "Skipped on Apple platform")
		skipIfVmtype(define.LibKrun, "Skipped on Apple platform")

		name := randomString()
		i := new(initMachine).withImage(mb.imagePath).withNow()

		// Empty arg forces no volumes
		i.withVolume("")
		session, err := mb.setName(name).setCmd(i).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		ssh9p := new(sshMachine).withSSHCommand([]string{"findmnt", "-no", "FSTYPE", "-t", "9p"})
		findmnt9p, err := mb.setName(name).setCmd(ssh9p).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(findmnt9p).To(Exit(0))
		Expect(findmnt9p.outputToString()).To(BeEmpty())

		sshVirtiofs := new(sshMachine).withSSHCommand([]string{"findmnt", "-no", "FSTYPE", "-t", "virtiofs"})
		findmntVirtiofs, err := mb.setName(name).setCmd(sshVirtiofs).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(findmntVirtiofs).To(Exit(0))
		Expect(findmntVirtiofs.outputToString()).To(BeEmpty())
	})

	It("Podman ops with port forwarding and gvproxy", func() {
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		ctrName := "test"
		bm := basicMachine{}
		runAlp, err := mb.setCmd(bm.withPodmanCommand([]string{"run", "-dt", "--name", ctrName, "-p", "62544:80", "quay.io/libpod/alpine_nginx"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(runAlp).To(Exit(0))
		testHTTPServer("62544", false, "podman rulez")

		// Test exec in machine scenario: https://github.com/containers/podman/issues/20821
		exec, err := mb.setCmd(bm.withPodmanCommand([]string{"exec", ctrName, "true"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(exec).To(Exit(0))

		out, err := pgrep("gvproxy")
		Expect(err).ToNot(HaveOccurred())
		Expect(out).ToNot(BeEmpty())

		rmCon, err := mb.setCmd(bm.withPodmanCommand([]string{"rm", "-af"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(rmCon).To(Exit(0))
		testHTTPServer("62544", true, "")

		stop := new(stopMachine)
		stopSession, err := mb.setCmd(stop).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopSession).To(Exit(0))

		// gxproxy should exit after machine is stopped
		out, _ = pgrep("gvproxy")
		Expect(out).ToNot(ContainSubstring("gvproxy"))
	})

	It("podman volume on non-standard path", func() {
		skipIfWSL("Requires standard volume handling")
		dir, err := os.MkdirTemp("", "machine-volume")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(dir)

		testString := "abcdefg1234567"
		testFile := "testfile"
		err = os.WriteFile(filepath.Join(dir, testFile), []byte(testString), 0644)
		Expect(err).ToNot(HaveOccurred())

		name := randomString()
		machinePath := "/does/not/exist"
		init := new(initMachine).withVolume(fmt.Sprintf("%s:%s", dir, machinePath)).withImage(mb.imagePath).withNow()
		session, err := mb.setName(name).setCmd(init).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		// Must use path.Join to ensure forward slashes are used, even on Windows.
		ssh := new(sshMachine).withSSHCommand([]string{"cat", path.Join(machinePath, testFile)})
		ls, err := mb.setName(name).setCmd(ssh).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(ls).To(Exit(0))
		Expect(ls.outputToString()).To(ContainSubstring(testString))
	})

	It("podman build contexts", func() {
		skipIfVmtype(define.HyperVVirt, "FIXME: #23429 - Error running podman build with option --build-context on Hyper-V")
		skipIfVmtype(define.QemuVirt, "FIXME: #23433 - Additional build contexts should be sent as additional tar files")
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		mainContextDir := GinkgoT().TempDir()
		cfile := filepath.Join(mainContextDir, "test1")
		err = os.WriteFile(cfile, []byte(name), 0o644)
		Expect(err).ToNot(HaveOccurred())

		additionalContextDir := GinkgoT().TempDir()
		cfile = filepath.Join(additionalContextDir, "test2")
		err = os.WriteFile(cfile, []byte(name), 0o644)
		Expect(err).ToNot(HaveOccurred())

		cfile = filepath.Join(mainContextDir, "Containerfile")
		err = os.WriteFile(cfile, []byte("FROM quay.io/libpod/alpine_nginx\nCOPY test1 /\nCOPY --from=test-context test2 /\n"), 0o644)
		Expect(err).ToNot(HaveOccurred())

		bm := basicMachine{}
		build, err := mb.setCmd(bm.withPodmanCommand([]string{"build", "-t", name, "--build-context", "test-context=" + additionalContextDir, mainContextDir})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(build).To(Exit(0))
		Expect(build.outputToString()).To(ContainSubstring("COMMIT"))

		run, err := mb.setCmd(bm.withPodmanCommand([]string{"run", name, "cat", "/test1"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(run).To(Exit(0))
		Expect(build.outputToString()).To(ContainSubstring(name))

		run, err = mb.setCmd(bm.withPodmanCommand([]string{"run", name, "cat", "/test2"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(run).To(Exit(0))
		Expect(build.outputToString()).To(ContainSubstring(name))
	})
})

func testHTTPServer(port string, shouldErr bool, expectedResponse string) {
	address := url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort("localhost", port),
	}

	interval := 250 * time.Millisecond
	var err error
	var resp *http.Response
	for i := 0; i < 6; i++ {
		resp, err = http.Get(address.String())
		if err != nil && shouldErr {
			Expect(err.Error()).To(ContainSubstring(expectedResponse))
			return
		}
		if err == nil {
			defer resp.Body.Close()
			break
		}
		time.Sleep(interval)
		interval *= 2
	}
	Expect(err).ToNot(HaveOccurred())

	body, err := io.ReadAll(resp.Body)
	Expect(err).ToNot(HaveOccurred())
	Expect(string(body)).Should(Equal(expectedResponse))
}
