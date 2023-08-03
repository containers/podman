package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/common/libimage"
	podmanRegistry "github.com/containers/podman/v4/hack/podman-registry-go"
	. "github.com/containers/podman/v4/test/utils"
	"github.com/containers/storage/pkg/archive"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// Internal function to verify instance compression
func verifyInstanceCompression(descriptor []imgspecv1.Descriptor, compression string, arch string) bool {
	for _, instance := range descriptor {
		if instance.Platform.Architecture != arch {
			continue
		}
		if compression == "zstd" {
			// if compression is zstd annotations must contain
			val, ok := instance.Annotations["io.github.containers.compression.zstd"]
			if ok && val == "true" {
				return true
			}
		} else if len(instance.Annotations) == 0 {
			return true
		}
	}
	return false
}

var _ = Describe("Podman manifest", func() {

	const (
		imageList                      = "docker://registry.k8s.io/pause:3.1"
		imageListInstance              = "docker://registry.k8s.io/pause@sha256:f365626a556e58189fc21d099fc64603db0f440bff07f77c740989515c544a39"
		imageListARM64InstanceDigest   = "sha256:f365626a556e58189fc21d099fc64603db0f440bff07f77c740989515c544a39"
		imageListAMD64InstanceDigest   = "sha256:59eec8837a4d942cc19a52b8c09ea75121acc38114a2c68b98983ce9356b8610"
		imageListARMInstanceDigest     = "sha256:c84b0a3a07b628bc4d62e5047d0f8dff80f7c00979e1e28a821a033ecda8fe53"
		imageListPPC64LEInstanceDigest = "sha256:bcf9771c0b505e68c65440474179592ffdfa98790eb54ffbf129969c5e429990"
		imageListS390XInstanceDigest   = "sha256:882a20ee0df7399a445285361d38b711c299ca093af978217112c73803546d5e"
	)

	It("create w/o image and attempt push w/o dest", func() {
		for _, amend := range []string{"--amend", "-a"} {
			session := podmanTest.Podman([]string{"manifest", "create", "foo"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))

			session = podmanTest.Podman([]string{"manifest", "create", "foo"})
			session.WaitWithDefaultTimeout()
			Expect(session).To(ExitWithError())

			session = podmanTest.Podman([]string{"manifest", "push", "--all", "foo"})
			session.WaitWithDefaultTimeout()
			Expect(session).To(ExitWithError())
			// Push should actually fail since its not valid registry
			Expect(session.ErrorToString()).To(ContainSubstring("requested access to the resource is denied"))
			Expect(session.OutputToString()).To(Not(ContainSubstring("accepts 2 arg(s), received 1")))

			session = podmanTest.Podman([]string{"manifest", "create", amend, "foo"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))

			session = podmanTest.Podman([]string{"manifest", "rm", "foo"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
		}
	})

	It("create w/ image", func() {
		session := podmanTest.Podman([]string{"manifest", "create", "foo", imageList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("inspect", func() {
		session := podmanTest.Podman([]string{"manifest", "inspect", BB})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"manifest", "inspect", "quay.io/libpod/busybox"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// inspect manifest of single image
		session = podmanTest.Podman([]string{"manifest", "inspect", "quay.io/libpod/busybox@sha256:6655df04a3df853b029a5fac8836035ac4fab117800c9a6c4b69341bb5306c3d"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("add w/ inspect", func() {
		session := podmanTest.Podman([]string{"manifest", "create", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		id := strings.TrimSpace(string(session.Out.Contents()))

		session = podmanTest.Podman([]string{"manifest", "inspect", id})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"manifest", "add", "--arch=arm64", "foo", imageListInstance})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"manifest", "inspect", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(imageListARM64InstanceDigest))
	})

	It("add with new version", func() {
		// Following test must pass for both podman and podman-remote
		session := podmanTest.Podman([]string{"manifest", "create", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		id := strings.TrimSpace(string(session.Out.Contents()))

		session = podmanTest.Podman([]string{"manifest", "inspect", id})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"manifest", "add", "--os-version", "7.7.7", "foo", imageListInstance})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"manifest", "inspect", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring("7.7.7"))
	})

	It("tag", func() {
		session := podmanTest.Podman([]string{"manifest", "create", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "add", "foobar", "quay.io/libpod/busybox"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"tag", "foobar", "foobar2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "inspect", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session2 := podmanTest.Podman([]string{"manifest", "inspect", "foobar2"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(Exit(0))
		Expect(session2.OutputToString()).To(Equal(session.OutputToString()))
	})

	It("push with --add-compression", func() {
		if podmanTest.Host.Arch == "ppc64le" {
			Skip("No registry image for ppc64le")
		}
		if isRootless() {
			err := podmanTest.RestoreArtifact(REGISTRY_IMAGE)
			Expect(err).ToNot(HaveOccurred())
		}
		lock := GetPortLock("5000")
		defer lock.Unlock()
		session := podmanTest.Podman([]string{"run", "-d", "--name", "registry", "-p", "5000:5000", REGISTRY_IMAGE, "/entrypoint.sh", "/etc/docker/registry/config.yml"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		if !WaitContainerReady(podmanTest, "registry", "listening on", 20, 1) {
			Skip("Cannot start docker registry.")
		}

		session = podmanTest.Podman([]string{"build", "--platform", "linux/amd64", "-t", "imageone", "build/basicalpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"build", "--platform", "linux/arm64", "-t", "imagetwo", "build/basicalpine"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"manifest", "create", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "add", "foobar", "containers-storage:localhost/imageone:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "add", "foobar", "containers-storage:localhost/imagetwo:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		push := podmanTest.Podman([]string{"manifest", "push", "--all", "--add-compression", "zstd", "--tls-verify=false", "--remove-signatures", "foobar", "localhost:5000/list"})
		push.WaitWithDefaultTimeout()
		Expect(push).Should(Exit(0))
		output := push.ErrorToString()
		// 4 images must be pushed two for gzip and two for zstd
		Expect(output).To(ContainSubstring("Copying 4 images generated from 2 images in list"))

		session = podmanTest.Podman([]string{"run", "--rm", "--net", "host", "quay.io/skopeo/stable", "inspect", "--tls-verify=false", "--raw", "docker://localhost:5000/list:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		var index imgspecv1.Index
		inspectData := []byte(session.OutputToString())
		err := json.Unmarshal(inspectData, &index)
		Expect(err).ToNot(HaveOccurred())

		Expect(verifyInstanceCompression(index.Manifests, "zstd", "amd64")).Should(BeTrue())
		Expect(verifyInstanceCompression(index.Manifests, "zstd", "arm64")).Should(BeTrue())
		Expect(verifyInstanceCompression(index.Manifests, "gzip", "arm64")).Should(BeTrue())
		Expect(verifyInstanceCompression(index.Manifests, "gzip", "amd64")).Should(BeTrue())
	})

	It("add --all", func() {
		session := podmanTest.Podman([]string{"manifest", "create", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "add", "--all", "foo", imageList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "inspect", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(
			And(
				ContainSubstring(imageListAMD64InstanceDigest),
				ContainSubstring(imageListARMInstanceDigest),
				ContainSubstring(imageListARM64InstanceDigest),
				ContainSubstring(imageListPPC64LEInstanceDigest),
				ContainSubstring(imageListS390XInstanceDigest),
			))
	})

	It("add --annotation", func() {
		session := podmanTest.Podman([]string{"manifest", "create", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "add", "--annotation", "hoge", "foo", imageList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		Expect(session.ErrorToString()).To(ContainSubstring("no value given for annotation"))
		session = podmanTest.Podman([]string{"manifest", "add", "--annotation", "hoge=fuga", "foo", imageList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "inspect", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		var inspect libimage.ManifestListData
		err := json.Unmarshal(session.Out.Contents(), &inspect)
		Expect(err).ToNot(HaveOccurred())
		Expect(inspect.Manifests[0].Annotations).To(Equal(map[string]string{"hoge": "fuga"}))
	})

	It("add --os", func() {
		session := podmanTest.Podman([]string{"manifest", "create", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "add", "--os", "bar", "foo", imageList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "inspect", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(`"os": "bar"`))
	})

	It("annotate", func() {
		session := podmanTest.Podman([]string{"manifest", "create", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "add", "foo", imageListInstance})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "annotate", "--annotation", "hello=world", "--arch", "bar", "foo", imageListARM64InstanceDigest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "inspect", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(`"architecture": "bar"`))
		// Check added annotation
		Expect(session.OutputToString()).To(ContainSubstring(`"hello": "world"`))
	})

	It("remove digest", func() {
		session := podmanTest.Podman([]string{"manifest", "create", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "add", "--all", "foo", imageList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "inspect", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(ContainSubstring(imageListARM64InstanceDigest))
		session = podmanTest.Podman([]string{"manifest", "remove", "foo", imageListARM64InstanceDigest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "inspect", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.OutputToString()).To(
			And(
				ContainSubstring(imageListAMD64InstanceDigest),
				ContainSubstring(imageListARMInstanceDigest),
				ContainSubstring(imageListPPC64LEInstanceDigest),
				ContainSubstring(imageListS390XInstanceDigest),
				Not(
					ContainSubstring(imageListARM64InstanceDigest)),
			))
	})

	It("remove not-found", func() {
		session := podmanTest.Podman([]string{"manifest", "create", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "add", "foo", imageList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "remove", "foo", "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError())

		session = podmanTest.Podman([]string{"manifest", "rm", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("push --all", func() {
		SkipIfRemote("manifest push to dir not supported in remote mode")
		session := podmanTest.Podman([]string{"manifest", "create", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "add", "--all", "foo", imageList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		dest := filepath.Join(podmanTest.TempDir, "pushed")
		err := os.MkdirAll(dest, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			os.RemoveAll(dest)
		}()
		session = podmanTest.Podman([]string{"manifest", "push", "--all", "foo", "dir:" + dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		files, err := filepath.Glob(dest + string(os.PathSeparator) + "*")
		Expect(err).ShouldNot(HaveOccurred())
		check := SystemExec("sha256sum", files)
		check.WaitWithDefaultTimeout()
		Expect(check).Should(Exit(0))
		prefix := "sha256:"
		Expect(check.OutputToString()).To(
			And(
				ContainSubstring(strings.TrimPrefix(imageListAMD64InstanceDigest, prefix)),
				ContainSubstring(strings.TrimPrefix(imageListARMInstanceDigest, prefix)),
				ContainSubstring(strings.TrimPrefix(imageListPPC64LEInstanceDigest, prefix)),
				ContainSubstring(strings.TrimPrefix(imageListS390XInstanceDigest, prefix)),
				ContainSubstring(strings.TrimPrefix(imageListARM64InstanceDigest, prefix)),
			))
	})

	It("push", func() {
		SkipIfRemote("manifest push to dir not supported in remote mode")
		session := podmanTest.Podman([]string{"manifest", "create", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "add", "--all", "foo", imageList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		dest := filepath.Join(podmanTest.TempDir, "pushed")
		err := os.MkdirAll(dest, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			os.RemoveAll(dest)
		}()
		session = podmanTest.Podman([]string{"push", "foo", "dir:" + dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		files, err := filepath.Glob(dest + string(os.PathSeparator) + "*")
		Expect(err).ToNot(HaveOccurred())
		check := SystemExec("sha256sum", files)
		check.WaitWithDefaultTimeout()
		Expect(check).Should(Exit(0))

		prefix := "sha256:"
		Expect(check.OutputToString()).To(
			And(
				ContainSubstring(strings.TrimPrefix(imageListAMD64InstanceDigest, prefix)),
				ContainSubstring(strings.TrimPrefix(imageListARMInstanceDigest, prefix)),
				ContainSubstring(strings.TrimPrefix(imageListPPC64LEInstanceDigest, prefix)),
				ContainSubstring(strings.TrimPrefix(imageListS390XInstanceDigest, prefix)),
				ContainSubstring(strings.TrimPrefix(imageListARM64InstanceDigest, prefix)),
			))
	})

	It("push with compression-format and compression-level", func() {
		SkipIfRemote("manifest push to dir not supported in remote mode")
		session := podmanTest.Podman([]string{"pull", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		dockerfile := `FROM quay.io/libpod/alpine:latest
RUN touch /file
`
		podmanTest.BuildImage(dockerfile, "localhost/test", "false")

		session = podmanTest.Podman([]string{"manifest", "create", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"manifest", "add", "foo", "containers-storage:localhost/test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// Invalid compression format specified, it must fail
		tmpDir := filepath.Join(podmanTest.TempDir, "wrong-compression")
		session = podmanTest.Podman([]string{"manifest", "push", "--compression-format", "gzip", "--compression-level", "50", "foo", "oci:" + tmpDir})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		output := session.ErrorToString()
		Expect(output).To(ContainSubstring("invalid compression level"))

		dest := filepath.Join(podmanTest.TempDir, "pushed")
		err := os.MkdirAll(dest, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			os.RemoveAll(dest)
		}()
		session = podmanTest.Podman([]string{"push", "--compression-format=zstd", "foo", "oci:" + dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		foundZstdFile := false

		blobsDir := filepath.Join(dest, "blobs", "sha256")

		blobs, err := os.ReadDir(blobsDir)
		Expect(err).ToNot(HaveOccurred())

		for _, f := range blobs {
			blobPath := filepath.Join(blobsDir, f.Name())

			sourceFile, err := os.ReadFile(blobPath)
			Expect(err).ToNot(HaveOccurred())

			compressionType := archive.DetectCompression(sourceFile)
			if compressionType == archive.Zstd {
				foundZstdFile = true
				break
			}
		}
		Expect(foundZstdFile).To(BeTrue(), "found zstd file")
	})

	It("push progress", func() {
		SkipIfRemote("manifest push to dir not supported in remote mode")

		session := podmanTest.Podman([]string{"manifest", "create", "foo", imageList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		dest := filepath.Join(podmanTest.TempDir, "pushed")
		err := os.MkdirAll(dest, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			os.RemoveAll(dest)
		}()

		session = podmanTest.Podman([]string{"push", "foo", "-q", "dir:" + dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.ErrorToString()).To(BeEmpty())

		session = podmanTest.Podman([]string{"push", "foo", "dir:" + dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		output := session.ErrorToString()
		Expect(output).To(ContainSubstring("Writing manifest list to image destination"))
		Expect(output).To(ContainSubstring("Storing list signatures"))
	})

	It("authenticated push", func() {
		registryOptions := &podmanRegistry.Options{
			PodmanPath: podmanTest.PodmanBinary,
			PodmanArgs: podmanTest.MakeOptions(nil, false, false),
			Image:      "docker-archive:" + imageTarPath(REGISTRY_IMAGE),
		}

		// Special case for remote: invoke local podman, with all
		// network/storage/other args
		if IsRemote() {
			registryOptions.PodmanArgs = getRemoteOptions(podmanTest, nil)
		}
		registry, err := podmanRegistry.StartWithOptions(registryOptions)
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			err := registry.Stop()
			Expect(err).ToNot(HaveOccurred())
		}()

		session := podmanTest.Podman([]string{"manifest", "create", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"pull", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"tag", ALPINE, "localhost:" + registry.Port + "/alpine:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		push := podmanTest.Podman([]string{"push", "--tls-verify=false", "--creds=" + registry.User + ":" + registry.Password, "--format=v2s2", "localhost:" + registry.Port + "/alpine:latest"})
		push.WaitWithDefaultTimeout()
		Expect(push).Should(Exit(0))

		session = podmanTest.Podman([]string{"manifest", "add", "--tls-verify=false", "--creds=" + registry.User + ":" + registry.Password, "foo", "localhost:" + registry.Port + "/alpine:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		push = podmanTest.Podman([]string{"manifest", "push", "--tls-verify=false", "--creds=" + registry.User + ":" + registry.Password, "foo", "localhost:" + registry.Port + "/credstest"})
		push.WaitWithDefaultTimeout()
		Expect(push).Should(Exit(0))
		output := push.ErrorToString()
		Expect(output).To(ContainSubstring("Copying blob "))
		Expect(output).To(ContainSubstring("Copying config "))
		Expect(output).To(ContainSubstring("Writing manifest to image destination"))

		push = podmanTest.Podman([]string{"manifest", "push", "--compression-format=gzip", "--compression-level=2", "--tls-verify=false", "--creds=podmantest:wrongpasswd", "foo", "localhost:" + registry.Port + "/credstest"})
		push.WaitWithDefaultTimeout()
		Expect(push).To(ExitWithError())
		Expect(push.ErrorToString()).To(ContainSubstring(": authentication required"))

		// push --rm after pull image (#15033)
		push = podmanTest.Podman([]string{"manifest", "push", "--rm", "--tls-verify=false", "--creds=" + registry.User + ":" + registry.Password, "foo", "localhost:" + registry.Port + "/rmtest"})
		push.WaitWithDefaultTimeout()
		Expect(push).Should(Exit(0))

		session = podmanTest.Podman([]string{"images", "-q", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("push with error", func() {
		session := podmanTest.Podman([]string{"manifest", "push", "badsrcvalue", "baddestvalue"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError())
		Expect(session.ErrorToString()).NotTo(BeEmpty())
	})

	It("push --rm to local directory", func() {
		SkipIfRemote("manifest push to dir not supported in remote mode")
		session := podmanTest.Podman([]string{"manifest", "create", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "add", "foo", imageList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		dest := filepath.Join(podmanTest.TempDir, "pushed")
		err := os.MkdirAll(dest, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			os.RemoveAll(dest)
		}()
		session = podmanTest.Podman([]string{"manifest", "push", "--purge", "foo", "dir:" + dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "push", "-p", "foo", "dir:" + dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		Expect(session.ErrorToString()).To(ContainSubstring("retrieving local image from image name foo: foo: image not known"))
		session = podmanTest.Podman([]string{"images", "-q", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

		// push --rm after pull image (#15033)
		session = podmanTest.Podman([]string{"pull", "quay.io/libpod/testdigest_v2s2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"manifest", "create", "bar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "add", "bar", "quay.io/libpod/testdigest_v2s2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"manifest", "push", "--rm", "bar", "dir:" + dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		session = podmanTest.Podman([]string{"images", "-q", "bar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))

		session = podmanTest.Podman([]string{"manifest", "rm", "foo", "bar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError())
		Expect(session.ErrorToString()).To(ContainSubstring("foo: image not known"))
		Expect(session.ErrorToString()).To(ContainSubstring("bar: image not known"))
	})

	It("exists", func() {
		manifestList := "manifest-list"
		session := podmanTest.Podman([]string{"manifest", "create", manifestList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"manifest", "exists", manifestList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"manifest", "exists", "no-manifest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))
	})

	It("rm should not remove referenced images", func() {
		manifestList := "manifestlist"
		imageName := "quay.io/libpod/busybox"

		session := podmanTest.Podman([]string{"pull", imageName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"manifest", "create", manifestList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"manifest", "add", manifestList, imageName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		session = podmanTest.Podman([]string{"manifest", "rm", manifestList})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// image should still show up
		session = podmanTest.Podman([]string{"image", "exists", imageName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
	})

	It("manifest rm should not remove image and should be able to remove tagged manifest list", func() {
		// manifest rm should fail with `image is not a manifest list`
		session := podmanTest.Podman([]string{"manifest", "rm", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(125))
		Expect(session.ErrorToString()).To(ContainSubstring("image is not a manifest list"))

		manifestName := "testmanifest:sometag"
		session = podmanTest.Podman([]string{"manifest", "create", manifestName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// verify if manifest exists
		session = podmanTest.Podman([]string{"manifest", "exists", manifestName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// manifest rm should be able to remove tagged manifest list
		session = podmanTest.Podman([]string{"manifest", "rm", manifestName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		// verify that manifest should not exist
		session = podmanTest.Podman([]string{"manifest", "exists", manifestName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(1))
	})
})
