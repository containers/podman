package e2e_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman machine cp", func() {
	It("all tests", func() {
		// HOST FILE SYSTEM
		// ~/<ginkgo_tmp>
		//   * foo.txt
		//   - foo-dir
		//     * bar.txt
		//
		//   * guest-foo.txt
		//   - guest-foo-dir
		//     * bar.txt

		// GUEST FILE SYSTEM
		// ~/
		//   * foo.txt
		//   - foo-dir
		//     * bar.txt

		var (
			file     = "foo.txt"
			filePath = filepath.Join(GinkgoT().TempDir(), file)

			directory     = "foo-dir"
			directoryPath = filepath.Join(GinkgoT().TempDir(), directory)

			fileInDirectory     = "bar.txt"
			fileInDirectoryPath = filepath.Join(directoryPath, fileInDirectory)

			guestToHostFile = "guest-foo.txt"
			guestToHostDir  = "guest-foo-dir"
		)

		_, err := os.Create(filePath)
		Expect(err).ToNot(HaveOccurred())

		err = os.MkdirAll(directoryPath, 0755)
		Expect(err).ToNot(HaveOccurred())

		_, err = os.Create(fileInDirectoryPath)
		Expect(err).ToNot(HaveOccurred())

		name := randomString()
		initMachine := initMachine{}
		cp := cpMachine{}
		sshMachine := sshMachine{}

		By("host file to guest")
		session, err := mb.setName(name).setCmd(initMachine.withImage(mb.imagePath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		// copy the file into the guest
		session, err = mb.setCmd(cp.withQuiet().withSrc(filePath).withDest(name + ":~/" + file)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		// verify the guest has the file
		session, err = mb.setName(name).setCmd(sshMachine.withSSHCommand([]string{"ls"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))
		Expect(session.outputToString()).To(Equal(file))

		// try to copy the file to a location in the guest where permission will get denied
		session, err = mb.setCmd(cp.withQuiet().withSrc(filePath).withDest(name + ":/etc/tmp.txt")).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))
		Expect(session.errorToString()).To(ContainSubstring("scp: dest open \"/etc/tmp.txt\": Permission denied"))

		By("host directory to guest")
		// copy contents into the guest
		session, err = mb.setCmd(cp.withQuiet().withSrc(directoryPath).withDest(name + ":~/" + directory)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		// verify the content is in the guest
		session, err = mb.setName(name).setCmd(sshMachine.withSSHCommand([]string{"ls", directory})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))
		Expect(session.outputToString()).To(Equal(fileInDirectory))

		By("guest file to host")
		// copy contents to the host
		guestToHostFilePath := filepath.Join(GinkgoT().TempDir(), guestToHostFile)
		session, err = mb.setCmd(cp.withQuiet().withSrc(name + ":~/" + file).withDest(guestToHostFilePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))
		// check the contents are on the host
		Expect(guestToHostFilePath).To(BeARegularFile())

		// this test can only be run on Unix systems. Windows does not
		// strictly enforce read-only permissions specified by `os.MkdirAll()`,
		// so we cannot easily make a read-only directory.
		if runtime.GOOS != "windows" {
			// try to copy the file to a location on the host where permission will get denied
			hostDirPath := filepath.Join(GinkgoT().TempDir(), "test-guest-copy-dir")
			err = os.MkdirAll(hostDirPath, 0444)
			Expect(err).ToNot(HaveOccurred())
			hostFileInDirPath := filepath.Join(hostDirPath, file)
			session, err = mb.setCmd(cp.withQuiet().withSrc(name + ":~/" + file).withDest(hostFileInDirPath)).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(session).To(Exit(125))
			Expect(session.errorToString()).To(ContainSubstring(fmt.Sprintf("scp: open local \"%s\": Permission denied", hostFileInDirPath)))
		}

		By("guest directory to host")
		// copy contents to the host
		guestToHostDirPath := filepath.Join(GinkgoT().TempDir(), guestToHostDir)
		session, err = mb.setCmd(cp.withQuiet().withSrc(name + ":~/" + directory).withDest(guestToHostDirPath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))
		// check the contents are on the host
		Expect(filepath.Join(guestToHostDirPath, fileInDirectory)).To(BeARegularFile())

		By("attempt copying file to a new directory")
		// copy the file to a guest directory
		session, err = mb.setCmd(cp.withQuiet().withSrc(filePath).withDest(name + ":~/directory/")).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))
		Expect(session.errorToString()).To(ContainSubstring("scp: dest open \"directory/\": Failure"))

		// try copying a guest file to a host directory
		hostDirPath := filepath.Join(GinkgoT().TempDir(), "directory") + string(filepath.Separator)
		session, err = mb.setCmd(cp.withQuiet().withSrc(name + ":~/" + file).withDest(hostDirPath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))
		switch runtime.GOOS {
		case "windows":
			hostDirPath = filepath.ToSlash(hostDirPath)
			fallthrough
		case "darwin":
			Expect(session.errorToString()).To(ContainSubstring(fmt.Sprintf("scp: open local \"%s\": No such file or directory", hostDirPath)))
		case "linux":
			Expect(session.errorToString()).To(ContainSubstring(fmt.Sprintf("scp: open local \"%s\": Is a directory", hostDirPath)))
		}

		By("attempt copying directory to a file")
		// try copying a local directory to a guest file
		session, err = mb.setCmd(cp.withQuiet().withSrc(GinkgoT().TempDir() + string(filepath.Separator)).withDest(name + ":~/" + directory + "/" + fileInDirectory + "/")).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))
		Expect(session.errorToString()).To(ContainSubstring("/foo-dir/bar.txt\" exists but is not a directory"))

		// try copying the guest directory to a local file
		session, err = mb.setCmd(cp.withQuiet().withSrc(name + ":~/" + directory + "/").withDest(filePath + string(filepath.Separator))).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))

		hostLocalFilePath := filePath + string(filepath.Separator)
		switch runtime.GOOS {
		case "windows":
			hostLocalFilePath = filepath.ToSlash(hostLocalFilePath)
			Expect(session.errorToString()).To(ContainSubstring(fmt.Sprintf("scp: open local \"%s\": No such file or directory", hostLocalFilePath+fileInDirectory)))
		case "darwin":
			Expect(session.errorToString()).To(ContainSubstring(fmt.Sprintf("scp: mkdir %s: Not a directory", hostLocalFilePath)))
		case "linux":
			Expect(session.errorToString()).To(ContainSubstring(fmt.Sprintf("scp: open local \"%s\": Not a directory", hostLocalFilePath+fileInDirectory)))
		}
	})
})
