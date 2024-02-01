package integration

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v5/libpod/define"
	. "github.com/containers/podman/v5/test/utils"
	"github.com/containers/storage/pkg/stringid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman run", func() {

	It("podman run a container based on local image", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	// This test may seem entirely pointless, it is not.  Due to compatibility
	// and historical reasons, the container name generator uses a globally
	// scoped RNG, seeded from a global state.  An easy way to check if its
	// been initialized properly (i.e. pseudo-non-deterministically) is
	// checking if the name-generator spits out the same name twice.  Because
	// existing containers are checked when generating names, the test must ensure
	// the first container is removed before creating a second.
	It("podman generates different names for successive containers", func() {
		var names [2]string

		for i := range names {
			session := podmanTest.Podman([]string{"create", ALPINE, "true"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			cid := session.OutputToString()
			Expect(cid).To(Not(Equal("")))

			session = podmanTest.Podman([]string{"container", "inspect", "--format", "{{.Name}}", cid})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			names[i] = session.OutputToString()
			Expect(names[i]).To(Not(Equal("")))

			session = podmanTest.Podman([]string{"rm", cid})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
		}
		Expect(names[0]).ToNot(Equal(names[1]), "Podman generated duplicate successive container names, has the global RNG been seeded correctly?")
	})

	It("podman run check /run/.containerenv", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "cat", "/run/.containerenv"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(""))

		session = podmanTest.Podman([]string{"run", "--privileged", "--name=test1", ALPINE, "cat", "/run/.containerenv"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("name=\"test1\""))
		Expect(session.OutputToString()).To(ContainSubstring("image=\"" + ALPINE + "\""))

		session = podmanTest.Podman([]string{"run", "-v", "/:/host", ALPINE, "cat", "/run/.containerenv"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("graphRootMounted=1"))

		session = podmanTest.Podman([]string{"run", "-v", "/:/host", "--privileged", ALPINE, "cat", "/run/.containerenv"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("graphRootMounted=1"))
	})

	It("podman run from manifest list", func() {
		session := podmanTest.Podman([]string{"manifest", "create", "localhost/test:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"build", "-q", "-f", "build/Containerfile.with-platform", "--platform", "linux/amd64,linux/arm64", "--manifest", "localhost/test:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--platform", "linux/arm64", "localhost/test", "uname", "-a"})
		session.WaitWithDefaultTimeout()
		exitCode := session.ExitCode()
		// CI could either support requested platform or not, if it supports then output should contain `aarch64`
		// if not run should fail with a very specific error i.e `Exec format error` anything other than this should
		// be marked as failure of test.
		if exitCode == 0 {
			Expect(session.OutputToString()).To(ContainSubstring("aarch64"))
		} else {
			// crun says 'Exec', runc says 'exec'. Handle either.
			Expect(session.ErrorToString()).To(ContainSubstring("xec format error"))
		}
	})

	It("podman run a container based on a complex local image name", func() {
		imageName := strings.TrimPrefix(NGINX_IMAGE, "quay.io/")
		session := podmanTest.Podman([]string{"run", imageName, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ErrorToString()).ToNot(ContainSubstring("Trying to pull"))
		Expect(session).Should(ExitCleanly())
	})

	It("podman run --signature-policy", func() {
		session := podmanTest.Podman([]string{"run", "--pull=always", "--signature-policy", "/no/such/file", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"run", "--pull=always", "--signature-policy", "/etc/containers/policy.json", ALPINE})
		session.WaitWithDefaultTimeout()
		if IsRemote() {
			Expect(session).To(ExitWithError())
			Expect(session.ErrorToString()).To(ContainSubstring("unknown flag"))
		} else {
			Expect(session).Should(Exit(0))
			Expect(session.ErrorToString()).To(ContainSubstring("Getting image source signatures"))
		}
	})

	It("podman run --rm with --restart", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--restart", "", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--rm", "--restart", "no", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--rm", "--restart", "on-failure", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--rm", "--restart", "always", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"run", "--rm", "--restart", "unless-stopped", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman run a container based on on a short name with localhost", func() {
		tag := podmanTest.Podman([]string{"tag", NGINX_IMAGE, "localhost/libpod/alpine_nginx:latest"})
		tag.WaitWithDefaultTimeout()

		rmi := podmanTest.Podman([]string{"rmi", NGINX_IMAGE})
		rmi.WaitWithDefaultTimeout()

		session := podmanTest.Podman([]string{"run", "libpod/alpine_nginx:latest", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ErrorToString()).ToNot(ContainSubstring("Trying to pull"))
		Expect(session).Should(ExitCleanly())
	})

	It("podman container run a container based on on a short name with localhost", func() {
		tag := podmanTest.Podman([]string{"image", "tag", NGINX_IMAGE, "localhost/libpod/alpine_nginx:latest"})
		tag.WaitWithDefaultTimeout()

		rmi := podmanTest.Podman([]string{"image", "rm", NGINX_IMAGE})
		rmi.WaitWithDefaultTimeout()

		session := podmanTest.Podman([]string{"container", "run", "libpod/alpine_nginx:latest", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ErrorToString()).ToNot(ContainSubstring("Trying to pull"))
		Expect(session).Should(ExitCleanly())
	})

	It("podman run a container based on local image with short options", func() {
		session := podmanTest.Podman([]string{"run", "-dt", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run a container based on local image with short options and args", func() {
		// regression test for #714
		session := podmanTest.Podman([]string{"run", ALPINE, "find", "/etc", "-name", "hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("/etc/hosts"))
	})

	It("podman run --name X --hostname Y, both X and Y in /etc/hosts", func() {
		name := "test_container"
		hostname := "test_hostname"
		session := podmanTest.Podman([]string{"run", "--rm", "--name", name, "--hostname", hostname, ALPINE, "cat", "/etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(name))
		Expect(session.OutputToString()).To(ContainSubstring(hostname))
	})

	It("podman run a container based on remote image", func() {
		// Pick any image that is not in our cache
		session := podmanTest.Podman([]string{"run", "-dt", BB_GLIBC, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.ErrorToString()).To(ContainSubstring("Trying to pull " + BB_GLIBC))
		Expect(session.ErrorToString()).To(ContainSubstring("Writing manifest to image destination"))

	})

	It("podman run --tls-verify", func() {
		// 5000 is marked insecure in registries.conf, so --tls-verify=false
		// is a NOP. Pick any other port.
		port := "5050"
		lock := GetPortLock(port)
		defer lock.Unlock()
		session := podmanTest.Podman([]string{"run", "-d", "--name", "registry", "-p", port + ":5000", REGISTRY_IMAGE, "/entrypoint.sh", "/etc/docker/registry/config.yml"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		if !WaitContainerReady(podmanTest, "registry", "listening on", 20, 1) {
			Fail("Cannot start docker registry.")
		}

		pushedImage := "localhost:" + port + "/pushed" + strings.ToLower(RandomString(5)) + ":" + RandomString(8)
		push := podmanTest.Podman([]string{"push", "--tls-verify=false", ALPINE, pushedImage})
		push.WaitWithDefaultTimeout()
		Expect(push).To(Exit(0))
		Expect(push.ErrorToString()).To(ContainSubstring("Writing manifest to image destination"))

		run := podmanTest.Podman([]string{"run", pushedImage, "date"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(125))
		Expect(run.ErrorToString()).To(ContainSubstring("pinging container registry localhost:" + port))
		Expect(run.ErrorToString()).To(ContainSubstring("http: server gave HTTP response to HTTPS client"))

		run = podmanTest.Podman([]string{"run", "--tls-verify=false", pushedImage, "echo", "got here"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))
		Expect(run.OutputToString()).To(Equal("got here"))
		Expect(run.ErrorToString()).To(ContainSubstring("Trying to pull " + pushedImage))
	})

	It("podman run a container with a --rootfs", func() {
		rootfs := filepath.Join(tempdir, "rootfs")
		uls := filepath.Join("/", "usr", "local", "share")
		uniqueString := stringid.GenerateRandomID()
		testFilePath := filepath.Join(uls, uniqueString)
		tarball := filepath.Join(tempdir, "rootfs.tar")

		err := os.Mkdir(rootfs, 0770)
		Expect(err).ShouldNot(HaveOccurred())

		// Change image in predictable way to validate export
		csession := podmanTest.Podman([]string{"run", "--name", uniqueString, ALPINE,
			"/bin/sh", "-c", fmt.Sprintf("echo %s > %s", uniqueString, testFilePath)})
		csession.WaitWithDefaultTimeout()
		Expect(csession).Should(ExitCleanly())

		// Export from working container image guarantees working root
		esession := podmanTest.Podman([]string{"export", "--output", tarball, uniqueString})
		esession.WaitWithDefaultTimeout()
		Expect(esession).Should(ExitCleanly())
		Expect(tarball).Should(BeARegularFile())

		// N/B: This will lose any extended attributes like SELinux types
		GinkgoWriter.Printf("Extracting container root tarball\n")
		tarsession := SystemExec("tar", []string{"xf", tarball, "-C", rootfs})
		Expect(tarsession).Should(ExitCleanly())
		Expect(filepath.Join(rootfs, uls)).Should(BeADirectory())

		// Other tests confirm SELinux types, just confirm --rootfs is working.
		session := podmanTest.Podman([]string{"run", "-i", "--security-opt", "label=disable",
			"--rootfs", rootfs, "cat", testFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Validate changes made in original container and export
		stdoutLines := session.OutputToStringArray()
		Expect(stdoutLines).Should(HaveLen(1))
		Expect(stdoutLines[0]).Should(Equal(uniqueString))

		// The rest of these tests only work locally and not containerized
		if IsRemote() || os.Getenv("container") != "" {
			GinkgoWriter.Println("Bypassing subsequent tests due to remote or container environment")
			return
		}
		// Test --rootfs with an external overlay
		// use --rm to remove container and confirm if we did not leak anything
		osession := podmanTest.Podman([]string{"run", "-i", "--rm", "--security-opt", "label=disable",
			"--rootfs", rootfs + ":O", "cat", testFilePath})
		osession.WaitWithDefaultTimeout()
		Expect(osession).Should(ExitCleanly())
		Expect(osession.OutputToString()).To(Equal(uniqueString))

		// Test podman start stop with overlay
		osession = podmanTest.Podman([]string{"run", "--name", "overlay-foo", "--security-opt", "label=disable",
			"--rootfs", rootfs + ":O", "echo", "hello"})
		osession.WaitWithDefaultTimeout()
		Expect(osession).Should(ExitCleanly())
		Expect(osession.OutputToString()).To(Equal("hello"))

		osession = podmanTest.Podman([]string{"stop", "overlay-foo"})
		osession.WaitWithDefaultTimeout()
		Expect(osession).Should(ExitCleanly())

		startsession := podmanTest.Podman([]string{"start", "--attach", "overlay-foo"})
		startsession.WaitWithDefaultTimeout()
		Expect(startsession).Should(ExitCleanly())
		Expect(startsession.OutputToString()).To(Equal("hello"))

		// remove container for above test overlay-foo
		osession = podmanTest.Podman([]string{"rm", "overlay-foo"})
		osession.WaitWithDefaultTimeout()
		Expect(osession).Should(ExitCleanly())

		// Test --rootfs with an external overlay with --uidmap
		osession = podmanTest.Podman([]string{"run", "--uidmap", "0:1234:5678", "--rm", "--security-opt", "label=disable",
			"--rootfs", rootfs + ":O", "cat", "/proc/self/uid_map"})
		osession.WaitWithDefaultTimeout()
		Expect(osession).Should(ExitCleanly())
		Expect(osession.OutputToString()).To(Equal("0 1234 5678"))
	})

	It("podman run a container with --init", func() {
		session := podmanTest.Podman([]string{"run", "--name", "test", "--init", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		result := podmanTest.Podman([]string{"inspect", "test"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		conData := result.InspectContainerToJSON()
		Expect(conData[0]).To(HaveField("Path", define.ContainerInitPath))
		Expect(conData[0].Config.Annotations).To(HaveKeyWithValue("io.podman.annotations.init", "TRUE"))
	})

	It("podman run a container with --init and --init-path", func() {
		// Also bind-mount /dev (#14251).
		session := podmanTest.Podman([]string{"run", "-v", "/dev:/dev", "--name", "test", "--init", "--init-path", "/usr/libexec/podman/catatonit", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		result := podmanTest.Podman([]string{"inspect", "test"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		conData := result.InspectContainerToJSON()
		Expect(conData[0]).To(HaveField("Path", define.ContainerInitPath))
		Expect(conData[0].Config.Annotations).To(HaveKeyWithValue("io.podman.annotations.init", "TRUE"))
	})

	It("podman run a container without --init", func() {
		session := podmanTest.Podman([]string{"run", "--name", "test", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		result := podmanTest.Podman([]string{"inspect", "test"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		conData := result.InspectContainerToJSON()
		Expect(conData[0]).To(HaveField("Path", "ls"))
		Expect(conData[0].Config.Annotations).To(Not(HaveKey("io.podman.annotations.init")))
	})

	forbidGetCWDSeccompProfile := func() string {
		in := []byte(`{"defaultAction":"SCMP_ACT_ALLOW","syscalls":[{"name":"getcwd","action":"SCMP_ACT_ERRNO"}]}`)
		jsonFile, err := podmanTest.CreateSeccompJSON(in)
		if err != nil {
			GinkgoWriter.Println(err)
			Skip("Failed to prepare seccomp.json for test.")
		}
		return jsonFile
	}

	It("podman run default mask test", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--name=maskCtr", ALPINE, "sleep", "200"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		for _, mask := range config.DefaultMaskedPaths {
			if st, err := os.Stat(mask); err == nil {
				if st.IsDir() {
					session = podmanTest.Podman([]string{"exec", "maskCtr", "ls", mask})
					session.WaitWithDefaultTimeout()
					Expect(session).Should(ExitCleanly())
					Expect(session.OutputToString()).To(BeEmpty())
				} else {
					session = podmanTest.Podman([]string{"exec", "maskCtr", "cat", mask})
					session.WaitWithDefaultTimeout()
					// Call can fail with permission denied, ignoring error or Not exist.
					// key factor is there is no information leak
					Expect(session.OutputToString()).To(BeEmpty())
				}
			}
		}
		for _, mask := range config.DefaultReadOnlyPaths {
			if _, err := os.Stat(mask); err == nil {
				session = podmanTest.Podman([]string{"exec", "maskCtr", "touch", mask})
				session.WaitWithDefaultTimeout()
				Expect(session).Should(Exit(1))
				Expect(session.ErrorToString()).To(Equal(fmt.Sprintf("touch: %s: Read-only file system", mask)))
			}
		}
	})

	It("podman run mask and unmask path test", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--name=maskCtr1", "--security-opt", "unmask=ALL", "--security-opt", "mask=/proc/acpi", ALPINE, "sleep", "200"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"exec", "maskCtr1", "ls", "/sys/firmware"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(Not(BeEmpty()))
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"exec", "maskCtr1", "ls", "/proc/acpi"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(BeEmpty())

		session = podmanTest.Podman([]string{"run", "-d", "--name=maskCtr2", "--security-opt", "unmask=/proc/acpi:/sys/firmware", ALPINE, "sleep", "200"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"exec", "maskCtr2", "ls", "/sys/firmware"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(Not(BeEmpty()))
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"exec", "maskCtr2", "ls", "/proc/acpi"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(Not(BeEmpty()))
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "-d", "--name=maskCtr3", "--security-opt", "mask=/sys/power/disk", ALPINE, "sleep", "200"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"exec", "maskCtr3", "cat", "/sys/power/disk"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(BeEmpty())
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "-d", "--name=maskCtr4", "--security-opt", "systempaths=unconfined", ALPINE, "sleep", "200"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"exec", "maskCtr4", "ls", "/sys/firmware"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(Not(BeEmpty()))
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "-d", "--name=maskCtr5", "--security-opt", "systempaths=unconfined", ALPINE, "grep", "/proc", "/proc/self/mounts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).Should(HaveLen(1))

		session = podmanTest.Podman([]string{"run", "-d", "--security-opt", "unmask=/proc/*", ALPINE, "grep", "/proc", "/proc/self/mounts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).Should(HaveLen(1))

		session = podmanTest.Podman([]string{"run", "--security-opt", "unmask=/proc/a*", ALPINE, "ls", "/proc/acpi"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Not(BeEmpty()))
	})

	It("podman run powercap is masked", func() {
		Skip("CI VMs do not have access to powercap")

		testCtr1 := "testctr"
		run := podmanTest.Podman([]string{"run", "-d", "--name", testCtr1, ALPINE, "top"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())

		exec := podmanTest.Podman([]string{"exec", "-ti", testCtr1, "ls", "/sys/devices/virtual/powercap"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).To(ExitWithError())

		testCtr2 := "testctr2"
		run2 := podmanTest.Podman([]string{"run", "-d", "--privileged", "--name", testCtr2, ALPINE, "top"})
		run2.WaitWithDefaultTimeout()
		Expect(run2).Should(ExitCleanly())

		exec2 := podmanTest.Podman([]string{"exec", "-ti", testCtr2, "ls", "/sys/devices/virtual/powercap"})
		exec2.WaitWithDefaultTimeout()
		Expect(exec2).Should(ExitCleanly())
		Expect(exec2.OutputToString()).Should(Not(BeEmpty()))
	})

	It("podman run security-opt unmask on /sys/fs/cgroup", func() {

		SkipIfCgroupV1("podman umask on /sys/fs/cgroup will fail with cgroups V1")
		SkipIfRootless("/sys/fs/cgroup rw access is needed")
		rwOnCgroups := "/sys/fs/cgroup cgroup2 rw"
		session := podmanTest.Podman([]string{"run", "--security-opt", "unmask=ALL", "--security-opt", "mask=/sys/fs/cgroup", ALPINE, "cat", "/proc/mounts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(rwOnCgroups))

		session = podmanTest.Podman([]string{"run", "--security-opt", "unmask=/sys/fs/cgroup", ALPINE, "cat", "/proc/mounts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(rwOnCgroups))

		session = podmanTest.Podman([]string{"run", "--security-opt", "unmask=/sys/fs/cgroup///", ALPINE, "cat", "/proc/mounts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(rwOnCgroups))

		session = podmanTest.Podman([]string{"run", "--security-opt", "unmask=ALL", ALPINE, "cat", "/proc/mounts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(rwOnCgroups))

		session = podmanTest.Podman([]string{"run", "--security-opt", "unmask=/sys/fs/cgroup", "--security-opt", "mask=/sys/fs/cgroup", ALPINE, "cat", "/proc/mounts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(rwOnCgroups))

		session = podmanTest.Podman([]string{"run", "--security-opt", "unmask=/sys/fs/cgroup", ALPINE, "ls", "/sys/fs/cgroup"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).ToNot(BeEmpty())
	})

	It("podman run seccomp test", func() {
		session := podmanTest.Podman([]string{"run", "--security-opt", strings.Join([]string{"seccomp=", forbidGetCWDSeccompProfile()}, ""), ALPINE, "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		Expect(session.ErrorToString()).To(ContainSubstring("Operation not permitted"))
	})

	It("podman run seccomp test --privileged", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", "--security-opt", strings.Join([]string{"seccomp=", forbidGetCWDSeccompProfile()}, ""), ALPINE, "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		Expect(session.ErrorToString()).To(ContainSubstring("Operation not permitted"))
	})

	It("podman run seccomp test --privileged no profile should be unconfined", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", ALPINE, "grep", "Seccomp", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(ContainSubstring("0"))
		Expect(session).Should(ExitCleanly())
	})

	It("podman run seccomp test no profile should be default", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "grep", "Seccomp", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(ContainSubstring("2"))
		Expect(session).Should(ExitCleanly())
	})

	It("podman run capabilities test", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--cap-add", "all", ALPINE, "cat", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--rm", "--cap-add", "sys_admin", ALPINE, "cat", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--rm", "--cap-drop", "all", ALPINE, "cat", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--rm", "--cap-drop", "setuid", ALPINE, "cat", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run user capabilities test", func() {
		// We need to ignore the containers.conf on the test distribution for this test
		os.Setenv("CONTAINERS_CONF", "/dev/null")
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}
		session := podmanTest.Podman([]string{"run", "--rm", "--user", "bin", ALPINE, "grep", "CapBnd", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("00000000800405fb"))

		session = podmanTest.Podman([]string{"run", "--rm", "--user", "bin", ALPINE, "grep", "CapEff", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("0000000000000000"))

		session = podmanTest.Podman([]string{"run", "--rm", "--user", "bin", ALPINE, "grep", "CapInh", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("0000000000000000"))

		session = podmanTest.Podman([]string{"run", "--rm", "--user", "root", ALPINE, "grep", "CapBnd", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("00000000800405fb"))

		session = podmanTest.Podman([]string{"run", "--rm", "--user", "root", ALPINE, "grep", "CapEff", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("00000000800405fb"))

		session = podmanTest.Podman([]string{"run", "--rm", "--user", "root", ALPINE, "grep", "CapInh", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("0000000000000000"))

		session = podmanTest.Podman([]string{"run", "--rm", ALPINE, "grep", "CapBnd", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("00000000800405fb"))

		session = podmanTest.Podman([]string{"run", "--rm", ALPINE, "grep", "CapEff", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("00000000800405fb"))

		session = podmanTest.Podman([]string{"run", "--user=1000:1000", "--cap-add=DAC_OVERRIDE", "--rm", ALPINE, "grep", "CapAmb", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("0000000000000002"))

		session = podmanTest.Podman([]string{"run", "--user=1000:1000", "--cap-add=DAC_OVERRIDE", "--rm", ALPINE, "grep", "CapInh", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("0000000000000002"))

		session = podmanTest.Podman([]string{"run", "--user=0", "--cap-add=DAC_OVERRIDE", "--rm", ALPINE, "grep", "CapAmb", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("0000000000000000"))

		session = podmanTest.Podman([]string{"run", "--user=0:0", "--cap-add=DAC_OVERRIDE", "--rm", ALPINE, "grep", "CapAmb", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("0000000000000000"))

		session = podmanTest.Podman([]string{"run", "--user=0:0", "--cap-add=DAC_OVERRIDE", "--rm", ALPINE, "grep", "CapInh", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("0000000000000000"))

		session = podmanTest.Podman([]string{"run", "--user=1:1", "--cap-add=DAC_OVERRIDE", "--rm", ALPINE, "grep", "CapEff", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("0000000000000002"))

		if isRootless() {
			if os.Getenv("SKIP_USERNS") != "" {
				GinkgoWriter.Println("Bypassing subsequent tests due to $SKIP_USERNS")
				return
			}
			if _, err := os.Stat("/proc/self/uid_map"); err != nil {
				GinkgoWriter.Println("Bypassing subsequent tests due to no /proc/self/uid_map")
				return
			}
			session = podmanTest.Podman([]string{"run", "--userns=keep-id", "--cap-add=DAC_OVERRIDE", "--rm", ALPINE, "grep", "CapAmb", "/proc/self/status"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(ContainSubstring("0000000000000002"))

			session = podmanTest.Podman([]string{"run", "--userns=keep-id", "--privileged", "--rm", ALPINE, "grep", "CapInh", "/proc/self/status"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(ContainSubstring("0000000000000000"))

			session = podmanTest.Podman([]string{"run", "--userns=keep-id", "--cap-add=DAC_OVERRIDE", "--rm", ALPINE, "grep", "CapInh", "/proc/self/status"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(ContainSubstring("0000000000000002"))
		}
	})

	It("podman run user capabilities test with image", func() {
		// We need to ignore the containers.conf on the test distribution for this test
		os.Setenv("CONTAINERS_CONF", "/dev/null")
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}
		dockerfile := fmt.Sprintf(`FROM %s
USER bin`, BB)
		podmanTest.BuildImage(dockerfile, "test", "false")
		session := podmanTest.Podman([]string{"run", "--rm", "--user", "bin", "test", "grep", "CapBnd", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("00000000800405fb"))

		session = podmanTest.Podman([]string{"run", "--rm", "--user", "bin", "test", "grep", "CapEff", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("0000000000000000"))
	})

	It("podman run limits test", func() {
		SkipIfRootlessCgroupsV1("Setting limits not supported on cgroupv1 for rootless users")

		if !isRootless() {
			session := podmanTest.Podman([]string{"run", "--rm", "--ulimit", "rtprio=99", "--cap-add=sys_nice", fedoraMinimal, "cat", "/proc/self/sched"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
		}

		session := podmanTest.Podman([]string{"run", "--rm", "--ulimit", "nofile=2048:2048", fedoraMinimal, "ulimit", "-n"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("2048"))

		session = podmanTest.Podman([]string{"run", "--rm", "--ulimit", "nofile=1024:1028", fedoraMinimal, "ulimit", "-n"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("1024"))

		if !CGROUPSV2 {
			// --oom-kill-disable not supported on cgroups v2.
			session = podmanTest.Podman([]string{"run", "--rm", "--oom-kill-disable=true", fedoraMinimal, "echo", "memory-hog"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
		}

		session = podmanTest.Podman([]string{"run", "--rm", "--oom-score-adj=111", fedoraMinimal, "cat", "/proc/self/oom_score_adj"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("111"))

		currentOOMScoreAdj, err := os.ReadFile("/proc/self/oom_score_adj")
		Expect(err).ToNot(HaveOccurred())
		name := "ctr-with-oom-score"
		session = podmanTest.Podman([]string{"create", "--name", name, fedoraMinimal, "cat", "/proc/self/oom_score_adj"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		for i := 0; i < 2; i++ {
			session = podmanTest.Podman([]string{"start", "-a", name})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal(strings.TrimRight(string(currentOOMScoreAdj), "\n")))
		}
	})

	It("podman run limits host test", func() {
		SkipIfRemote("This can only be used for local tests")

		var l syscall.Rlimit

		err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &l)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"run", "--rm", "--ulimit", "host", fedoraMinimal, "ulimit", "-Hn"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		ulimitCtrStr := strings.TrimSpace(session.OutputToString())
		ulimitCtr, err := strconv.ParseUint(ulimitCtrStr, 10, 0)
		Expect(err).ToNot(HaveOccurred())

		Expect(ulimitCtr).Should(BeNumerically(">=", l.Max))
	})

	It("podman run with cidfile", func() {
		session := podmanTest.Podman([]string{"run", "--cidfile", tempdir + "cidfile", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		err := os.Remove(tempdir + "cidfile")
		Expect(err).ToNot(HaveOccurred())
	})

	It("podman run sysctl test", func() {
		SkipIfRootless("Network sysctls are not available root rootless")
		session := podmanTest.Podman([]string{"run", "--rm", "--sysctl", "net.core.somaxconn=65535", ALPINE, "sysctl", "net.core.somaxconn"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("net.core.somaxconn = 65535"))

		// network sysctls should fail if --net=host is set
		session = podmanTest.Podman([]string{"run", "--net", "host", "--rm", "--sysctl", "net.core.somaxconn=65535", ALPINE, "sysctl", "net.core.somaxconn"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman run blkio-weight test", func() {
		SkipIfRootlessCgroupsV1("Setting blkio-weight not supported on cgroupv1 for rootless users")
		SkipIfRootless("By default systemd doesn't delegate io to rootless users")
		if CGROUPSV2 {
			if _, err := os.Stat("/sys/fs/cgroup/io.stat"); os.IsNotExist(err) {
				Skip("Kernel does not have io.stat")
			}
			if _, err := os.Stat("/sys/fs/cgroup/system.slice/io.bfq.weight"); os.IsNotExist(err) {
				Skip("Kernel does not support BFQ IO scheduler")
			}
			session := podmanTest.Podman([]string{"run", "--rm", "--blkio-weight=15", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/io.bfq.weight"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			// there was a documentation issue in the kernel that reported a different range [1-10000] for the io controller.
			// older versions of crun/runc used it.  For the time being allow both versions to pass the test.
			// FIXME: drop "|51" once all the runtimes we test have the fix in place.
			Expect(strings.Replace(session.OutputToString(), "default ", "", 1)).To(MatchRegexp("15|51"))
		} else {
			if _, err := os.Stat("/sys/fs/cgroup/blkio/blkio.weight"); os.IsNotExist(err) {
				Skip("Kernel does not support blkio.weight")
			}
			session := podmanTest.Podman([]string{"run", "--rm", "--blkio-weight=15", ALPINE, "cat", "/sys/fs/cgroup/blkio/blkio.weight"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(ContainSubstring("15"))
		}
	})

	It("podman run device-read-bps test", func() {
		SkipIfRootless("Setting device-read-bps not supported for rootless users")

		var session *PodmanSessionIntegration

		if CGROUPSV2 {
			session = podmanTest.Podman([]string{"run", "--rm", "--device-read-bps=/dev/zero:1mb", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/$(sed -e 's|0::||' < /proc/self/cgroup)/io.max"})
		} else {
			session = podmanTest.Podman([]string{"run", "--rm", "--device-read-bps=/dev/zero:1mb", ALPINE, "cat", "/sys/fs/cgroup/blkio/blkio.throttle.read_bps_device"})
		}

		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		if !CGROUPSV2 { // TODO: Test Simplification.  For now, we only care about exit(0) w/ cgroupsv2
			Expect(session.OutputToString()).To(ContainSubstring("1048576"))
		}
	})

	It("podman run device-write-bps test", func() {
		SkipIfRootless("Setting device-write-bps not supported for rootless users")

		var session *PodmanSessionIntegration

		if CGROUPSV2 {
			session = podmanTest.Podman([]string{"run", "--rm", "--device-write-bps=/dev/zero:1mb", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/$(sed -e 's|0::||' < /proc/self/cgroup)/io.max"})
		} else {
			session = podmanTest.Podman([]string{"run", "--rm", "--device-write-bps=/dev/zero:1mb", ALPINE, "cat", "/sys/fs/cgroup/blkio/blkio.throttle.write_bps_device"})
		}
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		if !CGROUPSV2 { // TODO: Test Simplification.  For now, we only care about exit(0) w/ cgroupsv2
			Expect(session.OutputToString()).To(ContainSubstring("1048576"))
		}
	})

	It("podman run device-read-iops test", func() {
		SkipIfRootless("Setting device-read-iops not supported for rootless users")
		var session *PodmanSessionIntegration

		if CGROUPSV2 {
			session = podmanTest.Podman([]string{"run", "--rm", "--device-read-iops=/dev/zero:100", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/$(sed -e 's|0::||' < /proc/self/cgroup)/io.max"})
		} else {
			session = podmanTest.Podman([]string{"run", "--rm", "--device-read-iops=/dev/zero:100", ALPINE, "cat", "/sys/fs/cgroup/blkio/blkio.throttle.read_iops_device"})
		}

		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		if !CGROUPSV2 { // TODO: Test Simplification.  For now, we only care about exit(0) w/ cgroupsv2
			Expect(session.OutputToString()).To(ContainSubstring("100"))
		}
	})

	It("podman run device-write-iops test", func() {
		SkipIfRootless("Setting device-write-iops not supported for rootless users")
		var session *PodmanSessionIntegration

		if CGROUPSV2 {
			session = podmanTest.Podman([]string{"run", "--rm", "--device-write-iops=/dev/zero:100", ALPINE, "sh", "-c", "cat /sys/fs/cgroup/$(sed -e 's|0::||' < /proc/self/cgroup)/io.max"})
		} else {
			session = podmanTest.Podman([]string{"run", "--rm", "--device-write-iops=/dev/zero:100", ALPINE, "cat", "/sys/fs/cgroup/blkio/blkio.throttle.write_iops_device"})
		}

		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		if !CGROUPSV2 { // TODO: Test Simplification.  For now, we only care about exit(0) w/ cgroupsv2
			Expect(session.OutputToString()).To(ContainSubstring("100"))
		}
	})

	It("podman run notify_socket", func() {
		SkipIfRemote("This can only be used for local tests")

		host := GetHostDistributionInfo()
		if host.Distribution != "rhel" && host.Distribution != "centos" && host.Distribution != "fedora" {
			Skip("this test requires a working runc")
		}
		sock := filepath.Join(podmanTest.TempDir, "notify")
		addr := net.UnixAddr{
			Name: sock,
			Net:  "unixgram",
		}
		socket, err := net.ListenUnixgram("unixgram", &addr)
		Expect(err).ToNot(HaveOccurred())
		defer os.Remove(sock)
		defer socket.Close()

		os.Setenv("NOTIFY_SOCKET", sock)
		defer os.Unsetenv("NOTIFY_SOCKET")

		session := podmanTest.Podman([]string{"run", ALPINE, "printenv", "NOTIFY_SOCKET"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).ToNot(BeEmpty())
	})

	It("podman run log-opt", func() {
		log := filepath.Join(podmanTest.TempDir, "/container.log")
		session := podmanTest.Podman([]string{"run", "--rm", "--log-driver", "k8s-file", "--log-opt", fmt.Sprintf("path=%s", log), ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		_, err := os.Stat(log)
		Expect(err).ToNot(HaveOccurred())
		_ = os.Remove(log)
	})

	It("podman run tagged image", func() {
		podmanTest.AddImageToRWStore(BB)
		tag := podmanTest.Podman([]string{"tag", BB, "bb"})
		tag.WaitWithDefaultTimeout()
		Expect(tag).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"run", "--rm", "bb", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman test hooks", func() {
		SkipIfRemote("--hooks-dir does not work with remote")
		hooksDir := tempdir + "/hooks,withcomma"
		err := os.Mkdir(hooksDir, 0755)
		Expect(err).ToNot(HaveOccurred())
		hookJSONPath := filepath.Join(hooksDir, "checkhooks.json")
		hookScriptPath := filepath.Join(hooksDir, "checkhooks.sh")
		targetFile := filepath.Join(hooksDir, "target")

		hookJSON := fmt.Sprintf(`{
	"cmd" : [".*"],
	"hook" : "%s",
	"stage" : [ "prestart" ]
}
`, hookScriptPath)
		err = os.WriteFile(hookJSONPath, []byte(hookJSON), 0644)
		Expect(err).ToNot(HaveOccurred())

		random := stringid.GenerateRandomID()

		hookScript := fmt.Sprintf(`#!/bin/sh
echo -n %s >%s
`, random, targetFile)
		err = os.WriteFile(hookScriptPath, []byte(hookScript), 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"--hooks-dir", hooksDir, "run", ALPINE, "ls"})
		session.Wait(10)
		Expect(session).Should(ExitCleanly())

		b, err := os.ReadFile(targetFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(b)).To(Equal(random))
	})

	It("podman run with subscription secrets", func() {
		SkipIfRemote("--default-mount-file option is not supported in podman-remote")
		containersDir := filepath.Join(podmanTest.TempDir, "containers")
		err := os.MkdirAll(containersDir, 0755)
		Expect(err).ToNot(HaveOccurred())

		secretsDir := filepath.Join(podmanTest.TempDir, "rhel", "secrets")
		err = os.MkdirAll(secretsDir, 0755)
		Expect(err).ToNot(HaveOccurred())

		mountsFile := filepath.Join(containersDir, "mounts.conf")
		mountString := secretsDir + ":/run/secrets"
		err = os.WriteFile(mountsFile, []byte(mountString), 0755)
		Expect(err).ToNot(HaveOccurred())

		secretsFile := filepath.Join(secretsDir, "test.txt")
		secretsString := "Testing secrets mount. I am mounted!"
		err = os.WriteFile(secretsFile, []byte(secretsString), 0755)
		Expect(err).ToNot(HaveOccurred())

		targetDir := tempdir + "/symlink/target"
		err = os.MkdirAll(targetDir, 0755)
		Expect(err).ToNot(HaveOccurred())
		keyFile := filepath.Join(targetDir, "key.pem")
		err = os.WriteFile(keyFile, []byte(mountString), 0755)
		Expect(err).ToNot(HaveOccurred())
		execSession := SystemExec("ln", []string{"-s", targetDir, filepath.Join(secretsDir, "mysymlink")})
		Expect(execSession).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"--default-mounts-file=" + mountsFile, "run", "--rm", ALPINE, "cat", "/run/secrets/test.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(secretsString))

		session = podmanTest.Podman([]string{"--default-mounts-file=" + mountsFile, "run", "--rm", ALPINE, "ls", "/run/secrets/mysymlink"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("key.pem"))
	})

	It("podman run with FIPS mode secrets", func() {
		SkipIfRootless("rootless can not manipulate system-fips file")
		fipsFile := "/etc/system-fips"
		err = os.WriteFile(fipsFile, []byte{}, 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "ls", "/run/secrets"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("system-fips"))

		err = os.Remove(fipsFile)
		Expect(err).ToNot(HaveOccurred())
	})

	It("podman run without group-add", func() {
		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "id"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Not(ContainSubstring("27(video),777,65533(nogroup)")))
	})

	It("podman run with group-add", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--group-add=audio", "--group-add=nogroup", "--group-add=777", ALPINE, "id"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("777,65533(nogroup)"))
	})

	It("podman run with user (default)", func() {
		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "id"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("uid=0(root) gid=0(root)"))
	})

	It("podman run with user (integer, not in /etc/passwd)", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--user=1234", ALPINE, "id"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("uid=1234(1234) gid=0(root) groups=0(root)"))
	})

	It("podman run with user (integer, in /etc/passwd)", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--user=8", ALPINE, "id"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("uid=8(mail) gid=12(mail)"))
	})

	It("podman run with user (username)", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--user=mail", ALPINE, "id"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("uid=8(mail) gid=12(mail)"))
	})

	It("podman run with user:group (username:integer)", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--user=mail:21", ALPINE, "id"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("uid=8(mail) gid=21(ftp) groups=21(ftp)"))
	})

	It("podman run with user:group (integer:groupname)", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--user=8:ftp", ALPINE, "id"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("uid=8(mail) gid=21(ftp) groups=21(ftp)"))
	})

	It("podman run with user, verify caps dropped", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--user=1234", ALPINE, "grep", "CapEff", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		capEff := strings.Split(session.OutputToString(), " ")
		Expect("0000000000000000").To(Equal(capEff[1]))
	})

	It("podman run with user, verify group added", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--user=1000:1000", ALPINE, "grep", "Groups:", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		groups := strings.Split(session.OutputToString(), " ")[1]
		Expect("1000").To(Equal(groups))
	})

	It("podman run with attach stdin outputs container ID", func() {
		session := podmanTest.Podman([]string{"run", "--attach", "stdin", ALPINE, "printenv"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		ps := podmanTest.Podman([]string{"ps", "-aq", "--no-trunc"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(ExitCleanly())
		Expect(ps.OutputToString()).To(ContainSubstring(session.OutputToString()))
	})

	It("podman run with attach stdout does not print stderr", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--attach", "stdout", ALPINE, "ls", "/doesnotexist"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(Equal(""))
	})

	It("podman run with attach stderr does not print stdout", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--attach", "stderr", ALPINE, "ls", "/"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(""))
	})

	It("podman run attach nonsense errors", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--attach", "asdfasdf", ALPINE, "ls", "/"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman run exit code on failure to exec", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "/etc"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(126))
	})

	It("podman run error on exec", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "sh", "-c", "exit 100"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(100))
	})

	It("podman run with named volume", func() {
		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "stat", "-c", "%a %Y", "/var/tmp"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		perms := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--rm", "-v", "test:/var/tmp", ALPINE, "stat", "-c", "%a %Y", "/var/tmp"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(perms))
	})

	It("podman run with built-in volume image", func() {
		session := podmanTest.Podman([]string{"run", "--rm", REDIS_IMAGE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		dockerfile := fmt.Sprintf(`FROM %s
RUN mkdir -p /myvol/data && chown -R mail.0 /myvol
VOLUME ["/myvol/data"]
USER mail`, BB)

		podmanTest.BuildImage(dockerfile, "test", "false")
		session = podmanTest.Podman([]string{"run", "--rm", "test", "ls", "-al", "/myvol/data"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("mail root"))
	})

	It("podman run --volumes-from flag", func() {
		vol := filepath.Join(podmanTest.TempDir, "vol-test")
		err := os.MkdirAll(vol, 0755)
		Expect(err).ToNot(HaveOccurred())

		filename := "test.txt"
		volFile := filepath.Join(vol, filename)
		data := "Testing --volumes-from!!!"
		err = os.WriteFile(volFile, []byte(data), 0755)
		Expect(err).ToNot(HaveOccurred())
		mountpoint := "/myvol/"

		session := podmanTest.Podman([]string{"create", "--volume", vol + ":" + mountpoint + ":z", ALPINE, "cat", mountpoint + filename})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		ctrID := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--volumes-from", ctrID, ALPINE, "cat", mountpoint + filename})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(data))

		session = podmanTest.Podman([]string{"run", "--volumes-from", ctrID, ALPINE, "sh", "-c", "echo test >> " + mountpoint + filename})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"start", "--attach", ctrID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(data + "test"))
	})

	It("podman run --volumes-from flag options", func() {
		vol := filepath.Join(podmanTest.TempDir, "vol-test")
		err := os.MkdirAll(vol, 0755)
		Expect(err).ToNot(HaveOccurred())

		filename := "test.txt"
		volFile := filepath.Join(vol, filename)
		data := "Testing --volumes-from!!!"
		err = os.WriteFile(volFile, []byte(data), 0755)
		Expect(err).ToNot(HaveOccurred())
		mountpoint := "/myvol/"

		session := podmanTest.Podman([]string{"create", "--volume", vol + ":" + mountpoint, ALPINE, "cat", mountpoint + filename})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		ctrID := session.OutputToString()

		// check that the read-only option works
		session = podmanTest.Podman([]string{"run", "--volumes-from", ctrID + ":ro", ALPINE, "touch", mountpoint + "abc.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))
		Expect(session.ErrorToString()).To(ContainSubstring("Read-only file system"))

		// check that both z and ro options work
		session = podmanTest.Podman([]string{"run", "--volumes-from", ctrID + ":ro,z", ALPINE, "cat", mountpoint + filename})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(data))

		// check that multiple ro/rw are not working
		session = podmanTest.Podman([]string{"run", "--volumes-from", ctrID + ":ro,rw", ALPINE, "cat", mountpoint + filename})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		Expect(session.ErrorToString()).To(ContainSubstring("cannot set ro or rw options more than once"))

		// check that multiple z options are not working
		session = podmanTest.Podman([]string{"run", "--volumes-from", ctrID + ":z,z,ro", ALPINE, "cat", mountpoint + filename})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		Expect(session.ErrorToString()).To(ContainSubstring("cannot set :z more than once in mount options"))

		// create new read-only volume
		session = podmanTest.Podman([]string{"create", "--volume", vol + ":" + mountpoint + ":ro", ALPINE, "cat", mountpoint + filename})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		ctrID = session.OutputToString()

		// check if the original volume was mounted as read-only that --volumes-from also mount it as read-only
		session = podmanTest.Podman([]string{"run", "--volumes-from", ctrID, ALPINE, "touch", mountpoint + "abc.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))
		Expect(session.ErrorToString()).To(ContainSubstring("Read-only file system"))
	})

	It("podman run --volumes-from flag with built-in volumes", func() {
		session := podmanTest.Podman([]string{"create", REDIS_IMAGE, "sh"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		ctrID := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--volumes-from", ctrID, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("data"))
	})

	It("podman run --volumes-from flag mount conflicts with image volume", func() {
		volPathOnHost := filepath.Join(podmanTest.TempDir, "myvol")
		err := os.MkdirAll(volPathOnHost, 0755)
		Expect(err).ToNot(HaveOccurred())

		imgName := "testimg"
		volPath := "/myvol/mypath"
		dockerfile := fmt.Sprintf(`FROM %s
RUN mkdir -p %s
VOLUME %s`, ALPINE, volPath, volPath)
		podmanTest.BuildImage(dockerfile, imgName, "false")

		ctr1 := "ctr1"
		run1 := podmanTest.Podman([]string{"run", "-d", "-v", fmt.Sprintf("%s:%s:z", volPathOnHost, volPath), "--name", ctr1, ALPINE, "top"})
		run1.WaitWithDefaultTimeout()
		Expect(run1).Should(ExitCleanly())

		testFile := "testfile1"
		ctr1Exec := podmanTest.Podman([]string{"exec", "-t", ctr1, "touch", fmt.Sprintf("%s/%s", volPath, testFile)})
		ctr1Exec.WaitWithDefaultTimeout()
		Expect(ctr1Exec).Should(ExitCleanly())

		run2 := podmanTest.Podman([]string{"run", "--volumes-from", ctr1, imgName, "ls", volPath})
		run2.WaitWithDefaultTimeout()
		Expect(run2).Should(ExitCleanly())
		Expect(run2.OutputToString()).To(Equal(testFile))
	})

	It("podman run --volumes flag with multiple volumes", func() {
		vol1 := filepath.Join(podmanTest.TempDir, "vol-test1")
		err := os.MkdirAll(vol1, 0755)
		Expect(err).ToNot(HaveOccurred())
		vol2 := filepath.Join(podmanTest.TempDir, "vol-test2")
		err = os.MkdirAll(vol2, 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"run", "--volume", vol1 + ":/myvol1:z", "--volume", vol2 + ":/myvol2:z", ALPINE, "touch", "/myvol2/foo.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run --volumes flag with empty host dir", func() {
		vol1 := filepath.Join(podmanTest.TempDir, "vol-test1")
		err := os.MkdirAll(vol1, 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"run", "--volume", ":/myvol1:z", ALPINE, "touch", "/myvol2/foo.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		Expect(session.ErrorToString()).To(ContainSubstring("directory cannot be empty"))
		session = podmanTest.Podman([]string{"run", "--volume", vol1 + ":", ALPINE, "touch", "/myvol2/foo.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		Expect(session.ErrorToString()).To(ContainSubstring("directory cannot be empty"))
	})

	It("podman run --mount flag with multiple mounts", func() {
		vol1 := filepath.Join(podmanTest.TempDir, "vol-test1")
		err := os.MkdirAll(vol1, 0755)
		Expect(err).ToNot(HaveOccurred())
		vol2 := filepath.Join(podmanTest.TempDir, "vol-test2")
		err = os.MkdirAll(vol2, 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"run", "--mount", "type=bind,src=" + vol1 + ",target=/myvol1,z", "--mount", "type=bind,src=" + vol2 + ",target=/myvol2,z", ALPINE, "touch", "/myvol2/foo.txt"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run findmnt nothing shared", func() {
		vol1 := filepath.Join(podmanTest.TempDir, "vol-test1")
		err := os.MkdirAll(vol1, 0755)
		Expect(err).ToNot(HaveOccurred())
		vol2 := filepath.Join(podmanTest.TempDir, "vol-test2")
		err = os.MkdirAll(vol2, 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"run", "--volume", vol1 + ":/myvol1:z", "--volume", vol2 + ":/myvol2:z", fedoraMinimal, "findmnt", "-o", "TARGET,PROPAGATION"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Not(ContainSubstring("shared")))
	})

	It("podman run findmnt shared", func() {
		vol := filepath.Join(podmanTest.TempDir, "vol-test")
		err := os.MkdirAll(vol, 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"run", "--volume", vol + ":/myvol:z", fedoraMinimal, "findmnt", "-no", "PROPAGATION", "/myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("private"))

		session = podmanTest.Podman([]string{"run", "--volume", vol + ":/myvol:shared,z", fedoraMinimal, "findmnt", "-no", "PROPAGATION", "/myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		if isRootless() {
			// we need to relax the rootless test because it can be "shared" only when the user owns the outer mount namespace.
			Expect(session.OutputToString()).To(ContainSubstring("shared"))
		} else {
			// make sure it's only shared (and not 'shared,*')
			Expect(session.OutputToString()).To(Equal("shared"))
		}
	})

	It("podman run --security-opts proc-opts=", func() {
		session := podmanTest.Podman([]string{"run", "--security-opt", "proc-opts=nosuid,exec", fedoraMinimal, "findmnt", "-noOPTIONS", "/proc"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output := session.OutputToString()
		Expect(output).To(ContainSubstring("nosuid"))
		Expect(output).To(Not(ContainSubstring("exec")))
	})

	It("podman run --mount type=bind,bind-nonrecursive", func() {
		SkipIfRootless("FIXME: rootless users are not allowed to mount bind-nonrecursive")
		session := podmanTest.Podman([]string{"run", "--mount", "type=bind,bind-nonrecursive,private,src=/sys,target=/host-sys", fedoraMinimal, "findmnt", "-nR", "/host-sys"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman run --mount type=devpts,target=/foo/bar", func() {
		session := podmanTest.Podman([]string{"run", "--mount", "type=devpts,target=/foo/bar", fedoraMinimal, "stat", "-f", "-c%T", "/foo/bar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(define.TypeDevpts))
	})

	It("podman run --mount type=devpts,target=/dev/pts with uid, gid and mode", func() {
		// runc doesn't seem to honor uid= so avoid testing it
		session := podmanTest.Podman([]string{"run", "-t", "--mount", "type=devpts,target=/dev/pts,uid=1000,gid=1001,mode=123", fedoraMinimal, "stat", "-c%g-%a", "/dev/pts/0"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("1001-123"))
	})

	It("podman run --pod automatically", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--pod", "new:foobar", ALPINE, "nc", "-l", "-p", "8686"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--pod", "foobar", ALPINE, "/bin/sh", "-c", "echo test | nc -w 1 127.0.0.1 8686"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		check := podmanTest.Podman([]string{"pod", "ps", "--no-trunc"})
		check.WaitWithDefaultTimeout()
		Expect(check.OutputToString()).To(ContainSubstring("foobar"))
	})

	It("podman run --pod new with hostname", func() {
		hostname := "abc"
		session := podmanTest.Podman([]string{"run", "--pod", "new:foobar", "--hostname", hostname, ALPINE, "cat", "/etc/hostname"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(hostname))
	})

	It("podman run --rm should work", func() {
		session := podmanTest.Podman([]string{"run", "--name", "test", "--rm", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"wait", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		numContainers := podmanTest.NumberOfContainers()
		Expect(numContainers).To(Equal(0))
	})

	It("podman run --rm failed container should delete itself", func() {
		session := podmanTest.Podman([]string{"run", "--name", "test", "--rm", ALPINE, "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		session = podmanTest.Podman([]string{"wait", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		numContainers := podmanTest.NumberOfContainers()
		Expect(numContainers).To(Equal(0))
	})

	It("podman run failed container should NOT delete itself", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		// If remote we could have a race condition
		session = podmanTest.Podman([]string{"wait", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		numContainers := podmanTest.NumberOfContainers()
		Expect(numContainers).To(Equal(1))
	})
	It("podman run readonly container should NOT mount /dev/shm read/only", func() {
		session := podmanTest.Podman([]string{"run", "--read-only", ALPINE, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		Expect(session.OutputToString()).To(Not(ContainSubstring("/dev/shm type tmpfs (ro,")))
	})

	It("podman run readonly container should NOT mount /run noexec", func() {
		session := podmanTest.Podman([]string{"run", "--read-only", ALPINE, "sh", "-c", "mount  | grep \"/run \""})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		Expect(session.OutputToString()).To(Not(ContainSubstring("noexec")))
	})

	It("podman run with bad healthcheck retries", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--health-cmd", "[\"foo\"]", "--health-retries", "0", ALPINE, "top"})
		session.Wait()
		Expect(session).To(ExitWithError())
		Expect(session.ErrorToString()).To(ContainSubstring("healthcheck-retries must be greater than 0"))
	})

	It("podman run with bad healthcheck timeout", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--health-cmd", "foo", "--health-timeout", "0s", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		Expect(session.ErrorToString()).To(ContainSubstring("healthcheck-timeout must be at least 1 second"))
	})

	It("podman run with bad healthcheck start-period", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--health-cmd", "foo", "--health-start-period", "-1s", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		Expect(session.ErrorToString()).To(ContainSubstring("healthcheck-start-period must be 0 seconds or greater"))
	})

	It("podman run with --add-host and --no-hosts fails", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--add-host", "test1:127.0.0.1", "--no-hosts", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman run with restart-policy always restarts containers", func() {
		testDir := filepath.Join(podmanTest.RunRoot, "restart-test")
		err := os.MkdirAll(testDir, 0755)
		Expect(err).ToNot(HaveOccurred())

		aliveFile := filepath.Join(testDir, "running")
		file, err := os.Create(aliveFile)
		Expect(err).ToNot(HaveOccurred())
		file.Close()

		session := podmanTest.Podman([]string{"run", "-dt", "--restart", "always", "-v", fmt.Sprintf("%s:/tmp/runroot:Z", testDir), ALPINE, "sh", "-c", "touch /tmp/runroot/ran && while test -r /tmp/runroot/running; do sleep 0.1s; done"})

		found := false
		testFile := filepath.Join(testDir, "ran")
		for i := 0; i < 30; i++ {
			time.Sleep(1 * time.Second)
			if _, err := os.Stat(testFile); err == nil {
				found = true
				err = os.Remove(testFile)
				Expect(err).ToNot(HaveOccurred())
				break
			}
		}
		Expect(found).To(BeTrue(), "found expected /ran file")

		err = os.Remove(aliveFile)
		Expect(err).ToNot(HaveOccurred())

		session.WaitWithDefaultTimeout()

		// 10 seconds to restart the container
		found = false
		for i := 0; i < 10; i++ {
			time.Sleep(1 * time.Second)
			if _, err := os.Stat(testFile); err == nil {
				found = true
				break
			}
		}
		Expect(found).To(BeTrue(), "found /ran file after restart")
	})

	It("podman run with restart policy does not restart on manual stop", func() {
		ctrName := "testCtr"
		ctr := podmanTest.Podman([]string{"run", "-dt", "--restart=always", "--name", ctrName, ALPINE, "top"})
		ctr.WaitWithDefaultTimeout()
		Expect(ctr).Should(ExitCleanly())

		stop := podmanTest.Podman([]string{"stop", ctrName})
		stop.WaitWithDefaultTimeout()
		Expect(stop).Should(ExitCleanly())

		// This is ugly, but I don't see a better way
		time.Sleep(10 * time.Second)

		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman run with cgroups=split", func() {
		SkipIfNotSystemd(podmanTest.CgroupManager, "do not test --cgroups=split if not running on systemd")
		SkipIfRootlessCgroupsV1("Disable cgroups not supported on cgroupv1 for rootless users")
		SkipIfRemote("--cgroups=split cannot be used in remote mode")

		checkLines := func(lines []string) {
			cgroup := ""
			for _, line := range lines {
				parts := strings.SplitN(line, ":", 3)
				if len(parts) < 2 {
					continue
				}
				if !CGROUPSV2 {
					// ignore unified on cgroup v1.
					// both runc and crun do not set it.
					// crun does not set named hierarchies.
					if parts[1] == "" || strings.Contains(parts[1], "name=") {
						continue
					}
				}
				if parts[2] == "/" {
					continue
				}
				if cgroup == "" {
					cgroup = parts[2]
					continue
				}
				Expect(cgroup).To(Equal(parts[2]))
			}
		}

		container := podmanTest.PodmanSystemdScope([]string{"run", "--rm", "--cgroups=split", ALPINE, "cat", "/proc/self/cgroup"})
		container.WaitWithDefaultTimeout()
		Expect(container).Should(Exit(0))
		checkLines(container.OutputToStringArray())
		Expect(container.ErrorToString()).To(Or(
			ContainSubstring("Running scope as unit: "), // systemd <  255
			ContainSubstring("Running as unit: ")))      // systemd >= 255

		// check that --cgroups=split is honored also when a container runs in a pod
		container = podmanTest.PodmanSystemdScope([]string{"run", "--rm", "--pod", "new:split-test-pod", "--cgroups=split", ALPINE, "cat", "/proc/self/cgroup"})
		container.WaitWithDefaultTimeout()
		Expect(container).Should(Exit(0))
		checkLines(container.OutputToStringArray())
		Expect(container.ErrorToString()).To(Or(
			ContainSubstring("Running scope as unit: "), // systemd <  255
			ContainSubstring("Running as unit: ")))      // systemd >= 255
	})

	It("podman run with cgroups=disabled runs without cgroups", func() {
		SkipIfRootlessCgroupsV1("Disable cgroups not supported on cgroupv1 for rootless users")
		// Only works on crun
		if !strings.Contains(podmanTest.OCIRuntime, "crun") {
			Skip("Test only works on crun")
		}

		ownsCgroup, err := cgroups.UserOwnsCurrentSystemdCgroup()
		Expect(err).ShouldNot(HaveOccurred())
		if !ownsCgroup {
			// Podman moves itself to a new cgroup if it doesn't own the current cgroup
			Skip("Test only works when Podman owns the current cgroup")
		}

		trim := func(i string) string {
			return strings.TrimSuffix(i, "\n")
		}

		curCgroupsBytes, err := os.ReadFile("/proc/self/cgroup")
		Expect(err).ShouldNot(HaveOccurred())
		curCgroups := trim(string(curCgroupsBytes))
		GinkgoWriter.Printf("Output:\n%s\n", curCgroups)
		Expect(curCgroups).ToNot(Equal(""))

		container := podmanTest.Podman([]string{"run", "--cgroupns=host", "--cgroups=disabled", ALPINE, "cat", "/proc/self/cgroup"})
		container.WaitWithDefaultTimeout()
		Expect(container).Should(ExitCleanly())

		ctrCgroups := trim(container.OutputToString())
		GinkgoWriter.Printf("Output\n:%s\n", ctrCgroups)

		Expect(ctrCgroups).To(Equal(curCgroups))
	})

	It("podman run with cgroups=enabled makes cgroups", func() {
		SkipIfRootlessCgroupsV1("Enable cgroups not supported on cgroupv1 for rootless users")
		// Only works on crun
		if !strings.Contains(podmanTest.OCIRuntime, "crun") {
			Skip("Test only works on crun")
		}

		curCgroupsBytes, err := os.ReadFile("/proc/self/cgroup")
		Expect(err).ToNot(HaveOccurred())
		var curCgroups = string(curCgroupsBytes)
		GinkgoWriter.Printf("Output:\n%s\n", curCgroups)
		Expect(curCgroups).To(Not(Equal("")))

		ctrName := "testctr"
		container := podmanTest.Podman([]string{"run", "--name", ctrName, "-d", "--cgroups=enabled", ALPINE, "top"})
		container.WaitWithDefaultTimeout()
		Expect(container).Should(ExitCleanly())

		// Get PID and get cgroups of that PID
		inspectOut := podmanTest.InspectContainer(ctrName)
		Expect(inspectOut).To(HaveLen(1))
		pid := inspectOut[0].State.Pid
		Expect(pid).To(Not(Equal(0)))

		ctrCgroupsBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
		Expect(err).ToNot(HaveOccurred())
		var ctrCgroups = string(ctrCgroupsBytes)
		GinkgoWriter.Printf("Output\n:%s\n", ctrCgroups)
		Expect(curCgroups).To(Not(Equal(ctrCgroups)))
	})

	It("podman run with cgroups=garbage errors", func() {
		session := podmanTest.Podman([]string{"run", "-d", "--cgroups=garbage", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman run should fail with nonexistent authfile", func() {
		session := podmanTest.Podman([]string{"run", "--authfile", "/tmp/nonexistent", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman run --device-cgroup-rule", func() {
		SkipIfRootless("rootless users are not allowed to mknod")
		deviceCgroupRule := "c 42:* rwm"
		session := podmanTest.Podman([]string{"run", "--cap-add", "mknod", "--name", "test", "-d", "--device-cgroup-rule", deviceCgroupRule, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"exec", "test", "mknod", "newDev", "c", "42", "1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run --replace", func() {
		// Make sure we error out with --name.
		session := podmanTest.Podman([]string{"create", "--replace", ALPINE, "/bin/sh"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))

		// Run and replace 5 times in a row the "same" container.
		ctrName := "testCtr"
		for i := 0; i < 5; i++ {
			session := podmanTest.Podman([]string{"run", "--detach", "--replace", "--name", ctrName, ALPINE, "top"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())

			// make sure Podman prints only one ID
			Expect(session.OutputToString()).To(HaveLen(64))
		}
	})

	It("podman run --preserve-fds", func() {
		devNull, err := os.Open("/dev/null")
		Expect(err).ToNot(HaveOccurred())
		defer devNull.Close()
		files := []*os.File{
			devNull,
		}
		session := podmanTest.PodmanExtraFiles([]string{"run", "--preserve-fds", "1", ALPINE, "ls"}, files)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run --preserve-fds invalid fd", func() {
		session := podmanTest.Podman([]string{"run", "--preserve-fds", "2", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		Expect(session.ErrorToString()).To(ContainSubstring("file descriptor 3 is not available"))
	})

	It("podman run --privileged and --group-add", func() {
		groupName := "mail"
		session := podmanTest.Podman([]string{"run", "--group-add", groupName, "--privileged", fedoraMinimal, "groups"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(groupName))
	})

	It("podman run --tz", func() {
		testDir := filepath.Join(podmanTest.RunRoot, "tz-test")
		err := os.MkdirAll(testDir, 0755)
		Expect(err).ToNot(HaveOccurred())

		tzFile := filepath.Join(testDir, "tzfile.txt")
		file, err := os.Create(tzFile)
		Expect(err).ToNot(HaveOccurred())
		defer os.Remove(tzFile)

		_, err = file.WriteString("Hello")
		Expect(err).ToNot(HaveOccurred())
		file.Close()

		badTZFile := fmt.Sprintf("../../../%s", tzFile)
		session := podmanTest.Podman([]string{"run", "--tz", badTZFile, "--rm", ALPINE, "date"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		Expect(session.ErrorToString()).To(
			Equal("Error: running container create option: finding timezone: time: invalid location name"))

		session = podmanTest.Podman([]string{"run", "--tz", "Pacific/Honolulu", "--rm", ALPINE, "date", "+'%H %Z'"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("HST"))

		session = podmanTest.Podman([]string{"run", "--tz", "local", "--rm", ALPINE, "date", "+'%H %Z'"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		t := time.Now()
		z, _ := t.Zone()
		h := strconv.Itoa(t.Hour())
		Expect(session.OutputToString()).To(ContainSubstring(z))
		Expect(session.OutputToString()).To(ContainSubstring(h))

	})

	It("podman run verify pids-limit", func() {
		SkipIfCgroupV1("pids-limit not supported on cgroup V1")
		limit := "4321"
		session := podmanTest.Podman([]string{"run", "--pids-limit", limit, "--net=none", "--rm", ALPINE, "cat", "/sys/fs/cgroup/pids.max"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(limit))
	})

	It("podman run umask", func() {
		if !strings.Contains(podmanTest.OCIRuntime, "crun") {
			Skip("Test only works on crun")
		}

		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "sh", "-c", "umask"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("0022"))

		session = podmanTest.Podman([]string{"run", "--umask", "0002", "--rm", ALPINE, "sh", "-c", "umask"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("0002"))

		session = podmanTest.Podman([]string{"run", "--umask", "0077", "--rm", fedoraMinimal, "umask"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("0077"))

		session = podmanTest.Podman([]string{"run", "--umask", "22", "--rm", ALPINE, "sh", "-c", "umask"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("0022"))

		session = podmanTest.Podman([]string{"run", "--umask", "9999", "--rm", ALPINE, "sh", "-c", "umask"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
		Expect(session.ErrorToString()).To(ContainSubstring("invalid umask"))
	})

	It("podman run makes workdir from image", func() {
		// BuildImage does not seem to work remote
		dockerfile := fmt.Sprintf(`FROM %s
WORKDIR /madethis`, BB)
		podmanTest.BuildImage(dockerfile, "test", "false")
		session := podmanTest.Podman([]string{"run", "--rm", "test", "pwd"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("/madethis"))
	})

	It("podman run --entrypoint does not use image command", func() {
		session := podmanTest.Podman([]string{"run", "--entrypoint", "/bin/echo", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// We can't guarantee the output is completely empty, some
		// nonprintables seem to work their way in.
		Expect(session.OutputToString()).To(Not(ContainSubstring("/bin/sh")))
	})

	It("podman run a container with log-level (lower case)", func() {
		session := podmanTest.Podman([]string{"--log-level=info", "run", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.ErrorToString()).To(ContainSubstring(" level=info "))
	})

	It("podman run a container with log-level (upper case)", func() {
		session := podmanTest.Podman([]string{"--log-level=INFO", "run", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.ErrorToString()).To(ContainSubstring(" level=info "))
	})

	It("podman run a container with --pull never should fail if no local store", func() {
		session := podmanTest.Podman([]string{"run", "--pull", "never", "docker.io/library/debian:latest", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
	})

	It("podman run container with --pull missing and only pull once", func() {
		session := podmanTest.Podman([]string{"run", "--pull", "missing", CIRROS_IMAGE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.ErrorToString()).To(ContainSubstring("Trying to pull"))

		session = podmanTest.Podman([]string{"run", "--pull", "missing", CIRROS_IMAGE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run container with --pull missing should pull image multiple times", func() {
		session := podmanTest.Podman([]string{"run", "--pull", "always", CIRROS_IMAGE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.ErrorToString()).To(ContainSubstring("Trying to pull"))

		session = podmanTest.Podman([]string{"run", "--pull", "always", CIRROS_IMAGE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.ErrorToString()).To(ContainSubstring("Trying to pull"))
	})

	It("podman run container with hostname and hostname environment variable", func() {
		hostnameEnv := "test123"
		session := podmanTest.Podman([]string{"run", "--hostname", "testctr", "--env", fmt.Sprintf("HOSTNAME=%s", hostnameEnv), ALPINE, "printenv", "HOSTNAME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(hostnameEnv))
	})

	It("podman run --secret", func() {
		secretsString := "somesecretdata"
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte(secretsString), 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "mysecret", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--secret", "mysecret", "--name", "secr", ALPINE, "cat", "/run/secrets/mysecret"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(secretsString))

		session = podmanTest.Podman([]string{"inspect", "secr", "--format", " {{(index .Config.Secrets 0).Name}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("mysecret"))

	})

	It("podman run --secret source=mysecret,type=mount", func() {
		secretsString := "somesecretdata"
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte(secretsString), 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "mysecret", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--secret", "source=mysecret,type=mount", "--name", "secr", ALPINE, "cat", "/run/secrets/mysecret"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(secretsString))

		session = podmanTest.Podman([]string{"inspect", "secr", "--format", " {{(index .Config.Secrets 0).Name}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("mysecret"))

	})

	It("podman run --secret source=mysecret,type=mount with target", func() {
		secretsString := "somesecretdata"
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte(secretsString), 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "mysecret_target", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--secret", "source=mysecret_target,type=mount,target=hello", "--name", "secr_target", ALPINE, "cat", "/run/secrets/hello"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(secretsString))

		session = podmanTest.Podman([]string{"inspect", "secr_target", "--format", " {{(index .Config.Secrets 0).Name}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("mysecret_target"))

	})

	It("podman run --secret source=mysecret,type=mount with target at /tmp", func() {
		secretsString := "somesecretdata"
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte(secretsString), 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "mysecret_target2", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--secret", "source=mysecret_target2,type=mount,target=/tmp/hello", "--name", "secr_target2", ALPINE, "cat", "/tmp/hello"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(secretsString))

		session = podmanTest.Podman([]string{"inspect", "secr_target2", "--format", " {{(index .Config.Secrets 0).Name}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("mysecret_target2"))

	})

	It("podman run --secret source=mysecret,type=env", func() {
		secretsString := "somesecretdata"
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte(secretsString), 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "mysecret", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--secret", "source=mysecret,type=env", "--name", "secr", ALPINE, "printenv", "mysecret"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(secretsString))
	})

	It("podman run --secret target option", func() {
		secretsString := "somesecretdata"
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte(secretsString), 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "mysecret", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--secret", "source=mysecret,type=env,target=anotherplace", "--name", "secr", ALPINE, "printenv", "anotherplace"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(secretsString))
	})

	It("podman run --secret mount with uid, gid, mode options", func() {
		secretsString := "somesecretdata"
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte(secretsString), 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "mysecret", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// check default permissions
		session = podmanTest.Podman([]string{"run", "--secret", "mysecret", "--name", "secr", ALPINE, "ls", "-l", "/run/secrets/mysecret"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output := session.OutputToString()
		Expect(output).To(ContainSubstring("-r--r--r--"))
		Expect(output).To(ContainSubstring("root"))

		session = podmanTest.Podman([]string{"run", "--secret", "source=mysecret,type=mount,uid=1000,gid=1001,mode=777", "--name", "secr2", ALPINE, "ls", "-ln", "/run/secrets/mysecret"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output = session.OutputToString()
		Expect(output).To(ContainSubstring("-rwxrwxrwx"))
		Expect(output).To(ContainSubstring("1000"))
		Expect(output).To(ContainSubstring("1001"))
	})

	It("podman run --secret with --user", func() {
		secretsString := "somesecretdata"
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte(secretsString), 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "mysecret", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--secret", "mysecret", "--name", "nonroot", "--user", "200:200", ALPINE, "cat", "/run/secrets/mysecret"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(secretsString))
	})

	It("podman run invalid secret option", func() {
		secretsString := "somesecretdata"
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte(secretsString), 0755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "mysecret", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Invalid type
		session = podmanTest.Podman([]string{"run", "--secret", "source=mysecret,type=other", "--name", "secr", ALPINE, "printenv", "mysecret"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		// Invalid option
		session = podmanTest.Podman([]string{"run", "--secret", "source=mysecret,invalid=invalid", "--name", "secr", ALPINE, "printenv", "mysecret"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		// Option syntax not valid
		session = podmanTest.Podman([]string{"run", "--secret", "source=mysecret,type", "--name", "secr", ALPINE, "printenv", "mysecret"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		// mount option with env type
		session = podmanTest.Podman([]string{"run", "--secret", "source=mysecret,type=env,uid=1000", "--name", "secr", ALPINE, "printenv", "mysecret"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		// No source given
		session = podmanTest.Podman([]string{"run", "--secret", "type=env", "--name", "secr", ALPINE, "printenv", "mysecret"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())
	})

	It("podman run --requires", func() {
		depName := "ctr1"
		depContainer := podmanTest.Podman([]string{"create", "--name", depName, ALPINE, "top"})
		depContainer.WaitWithDefaultTimeout()
		Expect(depContainer).Should(ExitCleanly())

		mainName := "ctr2"
		mainContainer := podmanTest.Podman([]string{"run", "--name", mainName, "--requires", depName, "-d", ALPINE, "top"})
		mainContainer.WaitWithDefaultTimeout()
		Expect(mainContainer).Should(ExitCleanly())

		stop := podmanTest.Podman([]string{"stop", "--all"})
		stop.WaitWithDefaultTimeout()
		Expect(stop).Should(ExitCleanly())

		start := podmanTest.Podman([]string{"start", mainName})
		start.WaitWithDefaultTimeout()
		Expect(start).Should(ExitCleanly())

		running := podmanTest.Podman([]string{"ps", "-q"})
		running.WaitWithDefaultTimeout()
		Expect(running).Should(ExitCleanly())
		Expect(running.OutputToStringArray()).To(HaveLen(2))
	})

	It("podman run with pidfile", func() {
		SkipIfRemote("pidfile not handled by remote")
		pidfile := tempdir + "pidfile"
		session := podmanTest.Podman([]string{"run", "--pidfile", pidfile, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		readFirstLine := func(path string) string {
			content, err := os.ReadFile(path)
			Expect(err).ToNot(HaveOccurred())
			return strings.Split(string(content), "\n")[0]
		}
		containerPID := readFirstLine(pidfile)
		_, err = strconv.Atoi(containerPID) // Make sure it's a proper integer
		Expect(err).ToNot(HaveOccurred())
	})

	It("podman run check personality support", func() {
		// TODO: Remove this as soon as this is merged and made available in our CI https://github.com/opencontainers/runc/pull/3126.
		if !strings.Contains(podmanTest.OCIRuntime, "crun") {
			Skip("Test only works on crun")
		}
		session := podmanTest.Podman([]string{"run", "--personality=LINUX32", "--name=testpersonality", ALPINE, "uname", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("i686"))
	})

	It("podman run /dev/shm has nosuid,noexec,nodev", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "grep", "/dev/shm", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output := session.OutputToString()
		Expect(output).To(ContainSubstring("nosuid"))
		Expect(output).To(ContainSubstring("noexec"))
		Expect(output).To(ContainSubstring("nodev"))
	})

	It("podman run and decrypt from local registry", func() {
		SkipIfRemote("Remote run does not support decryption")

		if podmanTest.Host.Arch == "ppc64le" {
			Skip("No registry image for ppc64le")
		}

		podmanTest.AddImageToRWStore(ALPINE)

		lock := GetPortLock("5000")
		defer lock.Unlock()
		session := podmanTest.Podman([]string{"run", "-d", "--name", "registry", "-p", "5000:5000", REGISTRY_IMAGE, "/entrypoint.sh", "/etc/docker/registry/config.yml"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		if !WaitContainerReady(podmanTest, "registry", "listening on", 20, 1) {
			Fail("Cannot start docker registry.")
		}

		bitSize := 1024
		keyFileName := filepath.Join(podmanTest.TempDir, "key,withcomma")
		publicKeyFileName, privateKeyFileName, err := WriteRSAKeyPair(keyFileName, bitSize)
		Expect(err).ToNot(HaveOccurred())

		imgPath := "localhost:5000/my-alpine"
		session = podmanTest.Podman([]string{"push", "--encryption-key", "jwe:" + publicKeyFileName, "--tls-verify=false", "--remove-signatures", ALPINE, imgPath})
		session.WaitWithDefaultTimeout()

		session = podmanTest.Podman([]string{"rmi", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Must fail without --decryption-key
		// NOTE: --tls-verify=false not needed, because localhost:5000 is in registries.conf
		session = podmanTest.Podman([]string{"run", imgPath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		Expect(session.ErrorToString()).To(ContainSubstring("Trying to pull " + imgPath))
		Expect(session.ErrorToString()).To(ContainSubstring("invalid tar header"))

		// With
		session = podmanTest.Podman([]string{"run", "--decryption-key", privateKeyFileName, imgPath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.ErrorToString()).To(ContainSubstring("Trying to pull " + imgPath))
	})

	It("podman run --shm-size-systemd", func() {
		ctrName := "testShmSizeSystemd"
		run := podmanTest.Podman([]string{"run", "--name", ctrName, "--shm-size-systemd", "10mb", "-d", SYSTEMD_IMAGE, "/sbin/init"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())

		logs := podmanTest.Podman([]string{"logs", ctrName})
		logs.WaitWithDefaultTimeout()
		Expect(logs).Should(ExitCleanly())

		mount := podmanTest.Podman([]string{"exec", ctrName, "mount"})
		mount.WaitWithDefaultTimeout()
		Expect(mount).Should(ExitCleanly())
		t, strings := mount.GrepString("tmpfs on /run/lock")
		Expect(t).To(BeTrue(), "found /run/lock")
		Expect(strings[0]).Should(ContainSubstring("size=10240k"))
	})

	It("podman run does not preserve image annotations", func() {
		annoName := "test.annotation.present"
		annoValue := "annovalue"
		imgName := "basicalpine"
		build := podmanTest.Podman([]string{"build", "-f", "build/basicalpine/Containerfile.with_label", "--annotation", fmt.Sprintf("%s=%s", annoName, annoValue), "-t", imgName})
		build.WaitWithDefaultTimeout()
		Expect(build).Should(ExitCleanly())
		Expect(build.ErrorToString()).To(BeEmpty(), "build error logged")

		ctrName := "ctr1"
		run := podmanTest.Podman([]string{"run", "-d", "--name", ctrName, imgName, "top"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		Expect(run.ErrorToString()).To(BeEmpty(), "run error logged")

		inspect := podmanTest.Podman([]string{"inspect", ctrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.ErrorToString()).To(BeEmpty(), "inspect error logged")

		inspectData := inspect.InspectContainerToJSON()
		Expect(inspectData).To(HaveLen(1))
		Expect(inspectData[0].Config.Annotations).To(Not(HaveKey(annoName)))
		Expect(inspectData[0].Config.Annotations).To(Not(HaveKey("testlabel")))
	})
})
