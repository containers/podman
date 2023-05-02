package utils_test

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Common functions test", func() {
	var defaultOSPath string
	var defaultCgroupPath string

	BeforeEach(func() {
		defaultOSPath = OSReleasePath
		defaultCgroupPath = ProcessOneCgroupPath
	})

	AfterEach(func() {
		OSReleasePath = defaultOSPath
		ProcessOneCgroupPath = defaultCgroupPath
	})

	It("Test CreateTempDirInTempDir", func() {
		tmpDir, _ := CreateTempDirInTempDir()
		_, err := os.Stat(tmpDir)
		Expect(os.IsNotExist(err)).ShouldNot(BeTrue(), "Directory is not created as expect")
	})

	It("Test SystemExec", func() {
		session := SystemExec(GoechoPath, []string{})
		Expect(session.Command.Process).ShouldNot(BeNil(), "SystemExec cannot start a process")
	})

	It("Test StringInSlice", func() {
		testSlice := []string{"apple", "peach", "pear"}
		Expect(StringInSlice("apple", testSlice)).To(BeTrue(), "apple should in ['apple', 'peach', 'pear']")
		Expect(StringInSlice("banana", testSlice)).ShouldNot(BeTrue(), "banana should not in ['apple', 'peach', 'pear']")
		Expect(StringInSlice("anything", []string{})).ShouldNot(BeTrue(), "anything should not in empty slice")
	})

	DescribeTable("Test GetHostDistributionInfo",
		func(path, id, ver string, empty bool) {
			txt := fmt.Sprintf("ID=%s\nVERSION_ID=%s", id, ver)
			if !empty {
				f, _ := os.Create(path)
				_, err := f.WriteString(txt)
				Expect(err).ToNot(HaveOccurred(), "Failed to write data.")
				f.Close()
			}

			OSReleasePath = path
			host := GetHostDistributionInfo()
			if empty {
				Expect(host).To(Equal(HostOS{}), "HostOs should be empty.")
			} else {
				Expect(host.Distribution).To(Equal(strings.Trim(id, "\"")))
				Expect(host.Version).To(Equal(strings.Trim(ver, "\"")))
			}
		},
		Entry("Configure file is not exist.", "/tmp/nonexistent", "", "", true),
		Entry("Item value with and without \"", "/tmp/os-release.test", "fedora", "\"28\"", false),
		Entry("Item empty with and without \"", "/tmp/os-release.test", "", "\"\"", false),
	)

	DescribeTable("Test IsKernelNewerThan",
		func(kv string, expect, isNil bool) {
			newer, err := IsKernelNewerThan(kv)
			Expect(newer).To(Equal(expect), "Version compare results is not as expect.")
			Expect(err == nil).To(Equal(isNil), "Error is not as expect.")
		},
		Entry("Invalid kernel version: 0", "0", false, false),
		Entry("Older kernel version:0.0", "0.0", true, true),
		Entry("Newer kernel version: 100.17.14", "100.17.14", false, true),
		Entry("Invalid kernel version: I am not a kernel version", "I am not a kernel version", false, false),
	)

	DescribeTable("Test TestIsCommandAvailable",
		func(cmd string, expect bool) {
			cmdExist := IsCommandAvailable(cmd)
			Expect(cmdExist).To(Equal(expect))
		},
		Entry("Command exist", GoechoPath, true),
		Entry("Command exist", "Fakecmd", false),
	)

	It("Test WriteJSONFile", func() {
		type testJSON struct {
			Item1 int
			Item2 []string
		}
		compareData := &testJSON{}

		testData := &testJSON{
			Item1: 5,
			Item2: []string{"test"},
		}

		testByte, err := json.Marshal(testData)
		Expect(err).ToNot(HaveOccurred(), "Failed to marshal data.")

		err = WriteJSONFile(testByte, "/tmp/testJSON")
		Expect(err).ToNot(HaveOccurred(), "Failed to write JSON to file.")

		read, err := os.Open("/tmp/testJSON")
		Expect(err).ToNot(HaveOccurred(), "Can not find the JSON file after we write it.")
		defer read.Close()

		bytes, err := io.ReadAll(read)
		Expect(err).ToNot(HaveOccurred())
		err = json.Unmarshal(bytes, compareData)
		Expect(err).ToNot(HaveOccurred())

		Expect(reflect.DeepEqual(testData, compareData)).To(BeTrue(), "Data changed after we store it to file.")
	})

	DescribeTable("Test Containerized",
		func(path string, setEnv, createFile, expect bool) {
			if setEnv && (os.Getenv("container") == "") {
				os.Setenv("container", "test")
				defer os.Setenv("container", "")
			}
			if !setEnv && (os.Getenv("container") != "") {
				containerized := os.Getenv("container")
				os.Setenv("container", "")
				defer os.Setenv("container", containerized)
			}
			txt := "1:test:/"
			if expect {
				txt = "2:docker:/"
			}
			if createFile {
				f, _ := os.Create(path)
				_, err := f.WriteString(txt)
				Expect(err).ToNot(HaveOccurred(), "Failed to write data.")
				f.Close()
			}
			ProcessOneCgroupPath = path
			Expect(Containerized()).To(Equal(expect))
		},
		Entry("Set container in env", "", true, false, true),
		Entry("Can not read from file", "/tmp/nonexistent", false, false, false),
		Entry("Docker in cgroup file", "/tmp/cgroup.test", false, true, true),
		Entry("Docker not in cgroup file", "/tmp/cgroup.test", false, true, false),
	)

	It("Test WriteRSAKeyPair", func() {
		fileName := "/tmp/test_key"
		bitSize := 1024

		publicKeyFileName, privateKeyFileName, err := WriteRSAKeyPair(fileName, bitSize)
		Expect(err).ToNot(HaveOccurred(), "Failed to write RSA key pair to files.")

		read, err := os.Open(publicKeyFileName)
		Expect(err).ToNot(HaveOccurred(), "Cannot find the public key file after we write it.")
		defer read.Close()

		read, err = os.Open(privateKeyFileName)
		Expect(err).ToNot(HaveOccurred(), "Cannot find the private key file after we write it.")
		defer read.Close()
	})

})
