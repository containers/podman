package e2e_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

const (
	kube = `
apiVersion: v1
kind: Pod
metadata:
  name: demo-build-remote
spec:
  containers:
    - name: container
      image: foobar
`
)

var _ = Describe("podman kube", func() {
	It("play build", func() {
		// init podman machine
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		// create a tmp directory
		contextDir := GinkgoT().TempDir()
		// create the yaml file which will be used
		kubeFile := filepath.Join(contextDir, "kube.yaml")
		err = os.WriteFile(kubeFile, []byte(kube), 0o644)
		Expect(err).ToNot(HaveOccurred())

		// create the foobar directory
		fooBarDir := filepath.Join(contextDir, "foobar")
		err = os.Mkdir(fooBarDir, 0o755)
		Expect(err).ToNot(HaveOccurred())

		// create the Containerfile for the foorbar image
		cfile := filepath.Join(fooBarDir, "Containerfile")
		err = os.WriteFile(cfile, []byte("FROM quay.io/libpod/alpine_nginx\nRUN ip addr\n"), 0o644)
		Expect(err).ToNot(HaveOccurred())

		// run the kube command with the build flag
		bm := basicMachine{}
		build, err := mb.setCmd(bm.withPodmanCommand([]string{"kube", "play", kubeFile, "--build"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(build).To(Exit(0))

		output := build.outputToString()
		Expect(output).To(ContainSubstring("Pod:"))
		Expect(output).To(ContainSubstring("Container:"))
	})
})
