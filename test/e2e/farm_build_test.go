//go:build linux || freebsd

// farm_build_test.go
//
// This module:
//   - prepares a simulated, multi-node test environment
//   - performs limited testing of the podman farm build functionality (see
//     farm_test.go for testing of farm maintenance operations.)
//
// Should the test environment set up fail, then the testing of the builds will
// skipped. This should not be interpreted as the build tests having failed.
//
// Testing of the farm build is functionality still a little limited, because the
// farm build does not, as yet, support emulated builds. Consequently, only builds
// of the native architecture be modelled. Hopefully this will be rectified shortly.
//
// The tests themselves all appear (in tabulated form) at the bottom of this file.
package integration

import (
	//	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/onsi/gomega/types"

	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

type ReverseProxy struct {
	srv          *http.Server
	lis          net.Listener
	proxyGotUsed bool
	pathPrefix   string
}

/*##########################################################################################################*/

func (rp ReverseProxy) Close() {
	// NB. Make sure you close in the right order.
	rp.srv.Close()
	rp.lis.Close()
}

/*##########################################################################################################*/

func (rp ReverseProxy) Url() string {
	return "tcp://" + rp.lis.Addr().String() + rp.pathPrefix
}

/*##########################################################################################################*/

func makeReverseProxy(remoteSocket string) (*ReverseProxy, error) {
	const pathPrefix = "/reverse/proxy/path/prefix"

	proxy := http.NewServeMux()

	srv := &http.Server{
		Handler:           proxy,
		ReadHeaderTimeout: time.Second,
	}

	// Serve the reverse proxy on a random port
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	rp := ReverseProxy{
		srv:          srv,
		lis:          lis,
		proxyGotUsed: false,
		pathPrefix:   pathPrefix}

	proxy.Handle(pathPrefix+"/", &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			rp.proxyGotUsed = true
			pr.Out.URL.Path = strings.TrimPrefix(pr.Out.URL.Path, pathPrefix)
			pr.Out.URL.RawPath = strings.TrimPrefix(pr.Out.URL.RawPath, pathPrefix)
			baseURL, _ := url.Parse("http://d")
			pr.SetURL(baseURL)
		},
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				By("Proxying to " + remoteSocket)
				url, err := url.Parse(remoteSocket)
				if err != nil {
					return nil, err
				}
				return (&net.Dialer{}).DialContext(ctx, "unix", url.Path)
			},
		},
	})

	go func() {
		// Runs in the background until the https server is
		// shutdown
		defer GinkgoRecover()
		Expect(srv.Serve(lis)).To(MatchError(http.ErrServerClosed))
	}()

	return &rp, nil
}

/*##########################################################################################################*/
/*##########################################################################################################*/
/*##########################################################################################################*/

var _ = Context("Testing farm build functionality :", Ordered, func() {

	// Note: there are two enclosed contexts:
	//
	//	the first Context builds the a read-only environment in which testing
	//  can be performed. It does not contain any test objects themselves
	//  and is NOT modified by the tests in any way. Consequently, it is ok to
	//  build it just once and retain it for the duration of all build tests.
	//
	//	the second Context performs the actual testing within that environment.
	//
	// Consequently, these two contexts must be performed in ORDER.

	const HOST_ARCH = "HOST_ARCH"
	const LOCAL_HOST = "(local)"
	const PROXY_URL = "PROXY_URL"
	const OFFLINE_URL = "OFFLINE_URL"
	const GOOD_SHORT_TAG = "GOOD_SHORT_TAG"
	const GOOD_LONG_TAG = "GOOD_LONG_TAG"
	const GOOD_VERY_LONG_TAG = "GOOD_VERY_LONG_TAG"

	type testImageDescriptor struct {
		image      string
		contextDir string
	}
	/*##########################################################################################################*/

	var prepareContextsDir = func(baseTmpDir string, testSrc testImageDescriptor) string {
		//
		// Creates:
		//  * if necessary, a root directory capable of holding a series of sub-directories.
		//
		//  * a subdirectory within that root dir holding, which hold a dockerfile with instructions for
		//    a simple  build upon the a given base image.
		//
		// Note: it returns the name of the root dir (NOT the subdir that it created.)

		contextsDir := filepath.Join(baseTmpDir, "contexts")
		err := os.Mkdir(contextsDir, 0755)
		Expect(err).ToNot(HaveOccurred())

		writeToDir := filepath.Join(contextsDir, testSrc.contextDir)
		err = os.Mkdir(writeToDir, 0755)
		Expect(err).ToNot(HaveOccurred())

		containerFileContents := fmt.Sprintf("FROM %s\nRUN arch | tee /arch.txt\nRUN date | tee /built.txt\n", testSrc.image)
		containerFile := writeToDir + "/Dockerfile"
		writeConf([]byte(containerFileContents), containerFile)

		return contextsDir
	}
	/*##########################################################################################################*/

	var setupConnectionConfigs = func(baseDir string) (string, string, string) {
		// Build two empty configuration files for local storage
		// of connection/farm data.
		connectionsDir, err := os.MkdirTemp(baseDir, "connections")
		Expect(err).ToNot(HaveOccurred())
		containersFile := filepath.Join(connectionsDir, "containers.conf")
		f, err := os.Create(containersFile)
		Expect(err).ToNot(HaveOccurred())
		f.Close()

		connectionsFile := filepath.Join(connectionsDir, "connections.conf")
		return connectionsDir, containersFile, connectionsFile
	}
	/*##########################################################################################################*/

	var setupStaticTest = func(inpDir string) (podmanTest *PodmanTestIntegration, err error) {

		// Here we set up a PodmanTestIntegrationObject for use in creating our test
		// environment. We are not using the podmanTest variable provided by the standard test
		// infrastructure, because we want both it and the objects it creates to live for the
		// entire duration of the testing. (podmanTest itself gets torn down and re-created for
		// each test.)

		// Cribbed from common_test.go
		tmpDir, err := os.MkdirTemp(inpDir, "subtest-")
		Expect(err).ToNot(HaveOccurred())
		podmanTempDir := filepath.Join(tmpDir, "p")
		err = os.Mkdir(podmanTempDir, 0o700)
		Expect(err).ToNot(HaveOccurred())

		// I think there is a subtl bug under PodmanTestCreateUtil somewhere. Somewhere it
		// does a check for Rootfulness, and sometimes it gets the answer wrong. I think if the user
		// is running in a group with a degree of elevated privilege, then it decides it
		// IS rootful. However it then tries to create a socket in a privileged area, only
		// to find it is not privileged enough for that...so it panics.

		// TODO: Need a check for rootfullness ( isRootless())before heading down this road. This only seems
		// to be a problem when running a test executable built from the localintegration target.
		// remoteintegration doesn;t seem to have a problem. Since (for reasons explained elsewhere),
		// we are always going to running a remoteintegration executable, this is not particularly
		// pressing. For now, we will de-escalate the panic.

		defer func() {
			// Catch the panic and handle a little more gracefully
			if e := recover(); e != nil {
				podmanTest = nil
				// Think this catches everything!
				// err = e.(error)
				err = fmt.Errorf("panicked while setting up staticTest")
			}
		}()
		podmanTest = PodmanTestCreateUtil(podmanTempDir, true)

		podmanTest.StartRemoteService()
		// this will create a podman server that will be activated by traffic on
		// its service.socket.

		// What does this do?
		podmanTest.Setup()

		return podmanTest, nil
	}
	/*##########################################################################################################*/

	// PodmanLocal
	//
	// PodmanLocal is essentially just the podmanTest structure that would have been
	// generated had the test executable been generated using the 'localintegration'
	// target rather then the `remoteintegration` target. Using it means, the 'Command'
	// method will generate 'podman' commands rather than 'podman-remote'
	//
	// Unfortunately, it needs its own cleanup method, because there is a bit of a bug
	// (I think) in the default Cleanup() associated with remote podmanTest. Should
	// the version of podmanTest you are cleaning up NOT actually be running a remote
	// server, and carry the various socket info associated with that, then the clean
	// up will panic.
	//
	// Our PodmanLocal hacks its way around that by stealing the necessary info
	// from a standard PodmanTest, and then starting its own server. It doesn't use
	// this server in anyway; its there just to stop the standard CleanUp() from
	// panicking.

	type PodmanLocal struct {
		*PodmanTestIntegration
		cleanup func()
	}

	var NewPodmanLocal = func(baseDir string) *PodmanLocal {
		tmpDir, err := os.MkdirTemp(baseDir, "subtest-")
		Expect(err).ToNot(HaveOccurred())

		podmanTempDir := filepath.Join(tmpDir, "p")
		err = os.Mkdir(podmanTempDir, 0o700)
		Expect(err).ToNot(HaveOccurred())

		pi := PodmanTestCreateUtil(podmanTempDir, false)

		return &PodmanLocal{
			PodmanTestIntegration: pi,
			cleanup: func() {
				// Create a `remote` podmanTest whose values we can steal
				// Note: it creates its files under the working directory of
				// pi, so will be automatically cleaned up when pi is cleaned up.
				var tmp_remote_podmanTest = PodmanTestCreateUtil(podmanTempDir, true)

				pi.RemoteSocket = tmp_remote_podmanTest.RemoteSocket
				pi.RemoteSocketLock = tmp_remote_podmanTest.RemoteSocketLock
				pi.StartRemoteService()
				pi.Setup()
				pi.Cleanup()
			},
		}
	}
	/*##########################################################################################################*/

	var standardTestImage = testImageDescriptor{
		image:      "quay.io/libpod/testimage:20241011",
		contextDir: "testImage20241011",
	}
	// This is the base image to be used for most of the tests. It is important that it is
	// a multi-arch manifest. This one contains the following archs:
	//   * linux/arm64,
	//   * linux/amd64,
	//   * linux/ppc64le,
	//   * linux/s390x"

	var goodTagBase = "localhost:5002/tst-"

	var connectionsConf string
	var containersConf string
	var containersDir string

	var hostArch string
	// var emuInfo string
	var testExe = "ginkgo"
	var err error
	var revProxy *ReverseProxy
	var proxyConnectionURL string

	var offlineRevProxy *ReverseProxy
	var offlineConnectionURL string

	var podmanStaticTest *PodmanTestIntegration
	var podmanStaticLocal *PodmanLocal
	var contextsDir string

	/*##########################################################################################################*/
	/*##########################################################################################################*/
	/*##########################################################################################################*/

	BeforeAll(func() {
		SkipIfNotRemote("requires podman API service")
		//
		// Important:
		//
		// Testing of the podman farm uses standard facilities that are only provided when the
		// test executable is built under the `remoteintegration` target. (i.e when the
		// test objects are provided by libpod_suite_remote_test.go)
		//
		// Consequently, and counter-intuitively, we need to test BOTH local (podman) and
		// remote (podman-remote) invocation of farm builds with a test executable built
		// by the `remoteintegration` make target
		//
		// We do not do ANY farm build testing in executables built using the localintegratinon
		// target.

		podmanStaticTest, err = setupStaticTest(GlobalTmpDir)
		Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%s failed to set up podmanTest struct", testExe))

		// We are going to use our own PodmanTestIntegration variable, rather than the one
		// (podmanTest) supplied in test_common.go. This is because the standard podmanTest
		// and its associated remote service is torn down and re-assembled afresh for each
		// individual test. We need  sockets and reverse proxy config, along with the
		// connection/farm configs to persist for the duration of all the build tests. This
		// should not pollute the testing in anyway, since once defined the test farms/connections
		// etc. are completely static.
		//
		// Note also that podamanStaticTest is ALWAYS created as a 'remote' instantiation because
		// we wish to use the remote server functionality that comes with it.  However, we will
		// use NOT the podmanCmd associated with its That is because, for a 'remote' instantiation
		// that will resolve to 'podman-remote'. In setting up the test environment we will always
		// want to use the 'podman' binary.

		// Temporary arrangement
		// At some point I expect this will get merged into the other Static struct....
		// ..then it will be able to use native commbed too.

		podmanStaticLocal = NewPodmanLocal(GlobalTmpDir)

		// Below we activate a couple of tcp ports on the server and create reverse proxies to redirect
		// traffic to the podman.socket. Note that there is no separate server process.
		// The reverse proxy function is performed by the test executable itself.  The reverse
		// proxies will be the target for the remote connections we are about to create.
		//
		// Below we create two:
		// revProxy: will remain open for the complete duration of the test session and will used in the
		// vast majority of the tests;
		//
		// * offlineRevProxy: will exist only as long as it takes to define a connection on it, and then
		//   it will be remove. The effect of this is to model a node in the Farm which is offline.
		//   It will be used in only a small number of tests.

		revProxy, err = makeReverseProxy(podmanStaticTest.RemoteSocket)
		Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%s failed to set up reverse proxy", testExe))
		proxyConnectionURL = revProxy.Url()

		offlineRevProxy, err = makeReverseProxy(podmanStaticTest.RemoteSocket)
		Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%s failed to set up reverse proxy", testExe))
		offlineConnectionURL = offlineRevProxy.Url()

		// Write out our Container files with instructions to build on our standard
		// test image
		contextsDir = prepareContextsDir(podmanStaticTest.TempDir, standardTestImage)

		containersDir, containersConf, connectionsConf = setupConnectionConfigs(podmanStaticTest.TempDir)

		hostArch, err = podmanStaticLocal.PodmanExitCleanly("info", "--format=json").jq(".host.os + \"/\" + .host.arch + \"/\" + .host.variant")
		if err != nil {
			Fail("unable to establish local host architecture. Cannot continue.")
		}
		// The jq command will return the hostArch enclosed with escaped quotes '\"', and if
		// there is not os variant in the output, with a trailing '/'. All these characters need
		// to be stripped.
		hostArch = strings.Trim(hostArch, "\\\"/")

		// emuInfo = podmanStaticLocal.PodmanExitCleanly("info", "--format", "{{json .Host.EmulatedArchitectures}}").OutputToString()
	})
	/*##########################################################################################################*/

	// Warning: If not codespell:ignored, the following gets kicked out by the
	// pull request CI checks.
	AfterAll(func() { // codespell:ignore afterall
		SkipIfNotRemote("requires podman API service")

		// All done, so tidy up the bits that won't get automatically
		// tidied up. NB. The order of clean up is important here.

		podmanStaticLocal.cleanup()
		revProxy.Close()

		// Clean up the directories we have created for our custom containers.conf,
		// connections.conf etc. If we don't, some of the PR CI checks will fail.
		os.RemoveAll(contextsDir)
		os.RemoveAll(containersDir)
		os.Unsetenv("PODMAN_CONNECTIONS_CONF")
		os.Unsetenv("CONTAINERS_CONF")

		podmanStaticTest.Cleanup()
	})
	/*##########################################################################################################*/
	/*##########################################################################################################*/
	/*##########################################################################################################*/

	Context("Preparing a static test env.   :", Ordered, func() {

		// This code get executed just once, when the tests are being prepared. It does
		// not get re-executed when the code is run. Whilst you can initialise variables
		// here, you would only really want to do that in pretty rare circumstances, and you
		// never want to try and initialise any actual test cases. If you do, restrict yourself
		// to initialising constants ONLY, and only because you want the value to be accessed in areas
		// where they are NOT in scope of one of the Setup Nodes e.g. table entries.

		BeforeEach(func() {
			// This gives each test visibility to the isolated
			// connection and farm objects we have created.
			os.Setenv("PODMAN_CONNECTIONS_CONF", connectionsConf)
			os.Setenv("CONTAINERS_CONF", containersConf)
		})
		/*##########################################################################################################*/

		AfterAll(func() { // codespell:ignore afterall
			// Here we effectively take one of our testing nodes offline
			// for the duration of the test.
			offlineRevProxy.Close()
		})
		/*##########################################################################################################*/

		DescribeTable("Creating Connections:",
			func(name string, url string, identity string) {
				// Setup a series of connections according to the table below. In setting up
				// the test env, we need to ensure we always use the podman command (i.e not
				// podman-remote which would be the default for podmanStaticTest).

				var cmd *exec.Cmd

				switch url {
				case OFFLINE_URL:
					{
						cmd = exec.Command(podmanStaticTest.PodmanBinary,
							"system", "connection", "add", name, offlineConnectionURL)
					}
				case PROXY_URL:
					{
						cmd = exec.Command(podmanStaticTest.PodmanBinary,
							"system", "connection", "add", name, proxyConnectionURL)
					}
				default:
					{
						cmd = exec.Command(podmanStaticTest.PodmanBinary,
							"system", "connection", "add", "--identity", identity, name, url)
					}
				}

				session, err := Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%q failed to execute", podmanStaticTest.PodmanBinary))
				Eventually(session, DefaultWaitTimeout).Should(Exit(0))
				Expect(session.Out.Contents()).Should(BeEmpty())
				Expect(session.Err.Contents()).Should(BeEmpty())

				// Now confirm that each connection is working, and
				// reporting the correct native architecture
				cmd = exec.Command(podmanStaticTest.PodmanBinary,
					"--connection", name, "info", "--format", "{{.Host.Arch}}",
				)
				session, err = Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%q failed to execute", podmanStaticTest.PodmanBinary))
				Eventually(session, DefaultWaitTimeout).Should(Exit(0))
				Expect(session.Err.Contents()).Should(BeEmpty())
			},
			// If you use variables in the Entry statements below, the value they will assume will the value they had
			// at the time the Container was evaluated, NOT the value they had at completion of any
			// set-up nodes, such as BeforeEach, AfterEach.

			Entry("Creating ConA    ", "ConA", PROXY_URL, ""),
			Entry("Creating ConB    ", "ConB", PROXY_URL, ""),
			Entry("Creating Default ", "Default", PROXY_URL, ""),
			Entry("Creating Offline ", "Offline", OFFLINE_URL, ""),
		)
		/*##########################################################################################################*/

		DescribeTable("Creating the farms     :",
			func(farmName string) {
				// Setup a series of connections according to the table below. In setting up
				// the test env, we need to ensure we always use the podman command (i.e not
				// podman-remote which would be the default for podmanStaticTest)

				cmd := exec.Command(podmanStaticTest.PodmanBinary,
					"farm", "create", farmName)

				session, err := Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%q failed to execute", podmanStaticTest.PodmanBinary))
				Eventually(session, DefaultWaitTimeout).Should(Exit(0))
				Expect(session.Err.Contents()).Should(BeEmpty())

				// Care! - we are not using <podmanTest>
				Expect(string(session.Out.Contents())).Should(Equal(fmt.Sprintf("Farm \"%s\" created\n", farmName)))
			},
			// If you use variables in the Entry statements below, the value they will assume will the value they had
			// at the time the Container was evaluated, NOT the value they had at completion of any
			// set-up nodes, such as BeforeEach, AfterEach.

			Entry("Creating defaultFarm  ", "defaultFarm"),
			Entry("Creating emptyFarm    ", "emptyFarm"),
			Entry("Creating offlineFarm  ", "offlineFarm"),
			Entry("Creating proxyFarm    ", "proxyFarm"),
			Entry("Creating multinodeFarm", "multinodeFarm"),
		)
		/*##########################################################################################################*/

		DescribeTable("Adding Connections  :",
			func(farmName string, connectionName string) {
				// Setup a series of connections according to the table below. In setting up
				// the test env, we need to ensure we always use the podman command (i.e not
				// podman-remote which would be the default for podmanStaticTest)

				cmd := exec.Command(podmanStaticTest.PodmanBinary,
					"farm", "update", "--add", connectionName, farmName)

				session, err := Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%q failed to execute", podmanStaticTest.PodmanBinary))
				Eventually(session, DefaultWaitTimeout).Should(Exit(0))
				Expect(session.Err.Contents()).Should(BeEmpty())

				// Care! - we are not using <podmanest>
				Expect(string(session.Out.Contents())).Should(Equal(fmt.Sprintf("Farm \"%s\" updated\n", farmName)))
			},
			// NOTE: If you use variables in the Entry statements below, the value they will assume
			// will the value they had at the time the ginkgo Container node was evaluated, NOT the value they
			// had at completion of any set-up nodes, such as BeforeEach, AfterEach.

			Entry("Adding ConA to proxyFarm     ", "proxyFarm", "ConA"),
			Entry("Adding ConA to offlineFarm   ", "offlineFarm", "ConA"),
			Entry("Adding Offline to offlineFarm", "offlineFarm", "Offline"),
			Entry("Adding ConA to multinodeFarm ", "multinodeFarm", "ConA"),
			Entry("Adding ConB to multinodeFarm ", "multinodeFarm", "ConB"),
			Entry("Adding Default to defaultFarm", "defaultFarm", "Default"),
		)
		/*##########################################################################################################*/
	})
	/*##########################################################################################################*/
	/*##########################################################################################################*/
	/*##########################################################################################################*/

	Describe("Performing farm build tests  :", func() {
		// Remember! : This section of code get executed just once, when the tests are being PREPARED. It does
		// not get re-executed when the code is EXECUTED. Whilst you can initialise variables
		// here, you would only really want to do that in pretty rare circumstances, and you
		// never want to try and initialise any actual test cases. If you do, restrict yourself
		// to initialising constants ONLY, and only because you want the value to be accessed in areas
		// where they are NOT in scope of one of the Setup Nodes e.g. table entries.

		var registryName string

		var podmanLocal *PodmanLocal

		type withTestScenarioOf struct {
			farm   string
			params string
			image  testImageDescriptor
			tag    string
		}

		type build struct {
			arch                string
			expectedTobeBuiltOn string
			usingEmulation      bool
			withCleanup         bool
		}

		type expectBuildsOf []build

		type expectFailureWith struct {
			message string
		}

		/*##########################################################################################################*/

		var runRegistry = func() string {
			// Set up a registry server running locally in a container. This has to be accessible both
			// locally and from our secondary podman servers.

			// NB Very occasionally, this port seems to get locked up and is not released when the test completes, meaning
			// all subsequent tests start failing. I'm not sure why, but a reboot of the VM will sort this out.
			lock := GetPortLock("5002")
			defer lock.Unlock()

			regName := "someRandomReg"

			cmd := exec.Command(podmanStaticTest.PodmanBinary, "run", "-d", "--replace", "--rm", "--name", regName, "-p", "5002:5000", REGISTRY_IMAGE, "/entrypoint.sh", "/etc/docker/registry/config.yml")

			session, err := Start(cmd, GinkgoWriter, GinkgoWriter)

			// I think this waiting 1 sec for listening on to appear in the log, and then repeating 20x
			// if !WaitContainerReady(podmanStaticTest, "registry", "listening on", 20, 1) {
			//    Skip("Cannot start docker registry.")
			// }
			Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%q failed to execute", podmanStaticTest.PodmanBinary))
			Eventually(session, DefaultWaitTimeout).Should(Exit(0))

			return regName
		}
		/*##########################################################################################################*/

		var stopRegistry = func(regName string) {
			cmd := exec.Command(podmanStaticTest.PodmanBinary, "stop", regName)

			session, err := Start(cmd, GinkgoWriter, GinkgoWriter)

			Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%q failed to execute", podmanStaticTest.PodmanBinary))
			Eventually(session, DefaultWaitTimeout).Should(Exit(0))
		}
		/*##########################################################################################################*/

		var skopeo = func(image string) *PodmanSessionIntegration {

			// skopeo inspect --tls-verify=false  --raw docker://img.pegortech.co.uk/fedora would typically produce output:
			//
			// {"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[{"mediaType":"appli\
			// cation/vnd.oci.image.manifest.v1+json","digest":"sha256:6dbd1777b7dfa0d6b595d3220cc05923eb9833d6e49263b65\
			// 32c0ea182765017","size":612,"platform":{"architecture":"arm64","os":"linux","variant":"v8"}},{"mediaType"\
			// :"application/vnd.oci.image.manifest.v1+json","digest":"sha256:6afa31388822c02a0452e190617affa6257d3c28ac5\
			// 4681ab0ab35193ab86df3","size":680,"platform":{"architecture":"amd64","os":"linux"}}]}

			var SkopeoBinary = "skopeo"

			var dockerImage = fmt.Sprintf("docker:%s%s", `//`, image)
			var skopeoInspect = []string{"inspect", "--tls-verify=false", "--raw", dockerImage}

			var session = SystemExec(SkopeoBinary, skopeoInspect)

			return &PodmanSessionIntegration{session}
		}
		/*##########################################################################################################*/

		// the fields below are accessed by ginkgo by reflection, so need to be exported (Capitalized)
		type Sessions struct {
			Bld      *PodmanSessionIntegration
			Manifest *PodmanSessionIntegration
			Skopeo   *PodmanSessionIntegration
		}
		/*##########################################################################################################*/

		var scanSessionForBuiltImageId = func(session *PodmanSessionIntegration, bldSpec build) (string, error) {
			// Function attempts to extract the Id of the image built for the given bld spec
			// from the stdout of the farm build (as captured in the session).
			// Returns an empty string if no match is found.

			var regExpEscape = func(inp string) string {
				// User input in the form of Connection names, could potentially contain regexp
				// metachars, which we will need to escape. There has to be a more elegant
				// way of going this!.
				inp = strings.ReplaceAll(inp, `\`, `\\`)
				for _, metachar := range []byte("(){}[].?+^$|*") {
					inp = strings.ReplaceAll(inp, string(metachar), `\`+string(metachar))
				}
				return inp
			}

			// Want to chop a max of two delimited items from the provided arch string
			// i.e. linux/amd64/v8/garbage --> linux, amd64, v8/garbage (though we assume there
			// will be no garbage!)

			remainder := bldSpec.arch
			opSys, remainder, _ := strings.Cut(remainder, "/")
			arch, variant, _ := strings.Cut(remainder, "/")

			// An example of the line we are trying to match is
			//    finished build for [{linux amd64 }] at "(local)": built 933faeeab52b2c22cf63881d032ce80eb17fe1c31fb82c6f5fab12f66a5887ca
			//
			// Our regexp match string to match it consists of two parts:
			// the bits where we DO want regexp metachars escaped (because they occur in the string we are matching);
			// and the bit containing the metachars we are actually trying to use.
			regExpStr := regExpEscape(fmt.Sprintf(`finished build for [{%s %s %s}] at "%s": built `, opSys, arch, variant, bldSpec.expectedTobeBuiltOn)) + `(\w+)`

			// Despite our efforts, it is possible that some user input could cause a dodgy regexpi to creep in
			// and fail. If so we need to bail.
			regExp, err := regexp.Compile(regExpStr)
			if err != nil {
				return "", err
			}

			matches := regExp.FindStringSubmatch(session.OutputToString())

			// If a match is found, we expect matches to contain exactly 2 values:
			// The full content of the entry matched; and the extracted value we are
			// after.
			foundImageId := ""
			err = nil
			for i, match := range matches {
				switch i {
				case 0:
					{
					}
				case 1:
					{
						foundImageId = match
					}
				default:
					{
						foundImageId = ""
						err = fmt.Errorf("Unexpectedly found multiple matches for ImageId")
					}
				}
			}

			return foundImageId, err
		}
		/*##########################################################################################################*/

		var indicateAnErrorFreeFarmBuild = func(imgRef string) types.GomegaMatcher {

			successMessage := fmt.Sprintf("Saved list to \"%s\"", imgRef)
			// Note we can't use exit cleanly, as the farm build writes routinely
			// writes stuff to stderr.
			return SatisfyAll(
				Exit(0),
				HaveField("OutputToString()", ContainSubstring(successMessage)),
			)
		}
		/*##########################################################################################################*/

		var thatShowsAnErrorFreeManifestCheck = func() types.GomegaMatcher {
			return SatisfyAll(
				ExitCleanly(),
			)
		}
		/*##########################################################################################################*/

		var thatShowsAnErrorFreeSkopeoCheck = func() types.GomegaMatcher {
			return SatisfyAll(
				ExitCleanly(),
			)
		}
		/*##########################################################################################################*/

		var indicateARunWithoutErrorsReported = func(imgRef string) types.GomegaMatcher {
			return And(
				HaveField("Bld", indicateAnErrorFreeFarmBuild(imgRef)),
				HaveField("Manifest", thatShowsAnErrorFreeManifestCheck()),
				HaveField("Skopeo", thatShowsAnErrorFreeSkopeoCheck()),
			)
		}
		/*##########################################################################################################*/

		var indicatesAGoodFarmBuildForSpec = func(bldSpec build) types.GomegaMatcher {
			// Checks whether the output from the 'farm build' indicates that it has
			// built something that conforms to the expected specification. Namely:
			// * That it has built the expected arch
			// * On the expected node
			// * whether that node was the local node.

			// Want to chop a max of two delimited items from the provided arch string
			// i.e. linux/amd64/v8/garbage --> linux, amd64, v8/garbage (though we assume there
			// will be no garbage!)
			remainder := bldSpec.arch
			opSys, remainder, _ := strings.Cut(remainder, "/")
			arch, variant, _ := strings.Cut(remainder, "/")

			startMessage := fmt.Sprintf("Starting build for [{%s %s %s}] at \"%s\"", opSys, arch, variant, bldSpec.expectedTobeBuiltOn)
			endMessage := fmt.Sprintf("finished build for [{%s %s %s}] at \"%s\": built", opSys, arch, variant, bldSpec.expectedTobeBuiltOn)

			return SatisfyAll(
				HaveField("OutputToString()", ContainSubstring(startMessage)),
				HaveField("OutputToString()", ContainSubstring(endMessage)),
			)
		}
		/*##########################################################################################################*/

		var indicatesCorrectNoOfImagesInTheBuild = func(expectedImagesBuilt int) types.GomegaMatcher {
			// Checks that the no of images built by the farm matches our expectation
			// For some reason this particular output is written to StdErr

			countMessage := fmt.Sprintf("Copying 0 images generated from %d images in list", expectedImagesBuilt)

			return (HaveField("ErrorToString()", ContainSubstring(countMessage)))
		}
		/*##########################################################################################################*/

		var indicateAFarmBuiltToSpecification = func(bldSpecs []build) types.GomegaMatcher {
			// Checks that the farm has built all the images exactly how
			// they were expected to be built (i.e as per the supplied set of buildSpecs). Also
			// checks that no other images were built that we weren't expecting to see.

			// Does this need to return TRUE if the list is empty?
			matchers := []types.GomegaMatcher{}

			for i, bldSpec := range bldSpecs {
				if i == 0 {
					// Only need to do this once and only if the array is not empty.
					matchers = append(matchers, indicatesCorrectNoOfImagesInTheBuild(len(bldSpecs)))
				}
				matchers = append(matchers, indicatesAGoodFarmBuildForSpec(bldSpec))
			}
			return And(matchers...)
		}
		/*##########################################################################################################*/

		var indicateAllImagesBuiltToSpecification = func(bldSpecs []build) types.GomegaMatcher {
			// Checks that the built images on th enodes have been cleared down, if the
			// --cleardown flag was supplied.
			//
			// If the images were to be retained, checks the relevant server for the presence
			// of the image and confirms that it of the right architecture.
			//
			// Note: it relies on the the array of build specs being the same size
			// as our podmanSession Array, and that the oreder of entries is the same.

			// Does this need to return TRUE if the list is empty?
			matchers := []types.GomegaMatcher{}

			for i, bldSpec := range bldSpecs {

				remainder := bldSpec.arch
				opSys, remainder, _ := strings.Cut(remainder, "/")
				arch, _, _ := strings.Cut(remainder, "/")

				if bldSpec.withCleanup {
					// Look for a failure message and infer that it means the image has been deleted.
					matchers = append(matchers, WithTransform(
						func(sessions []*PodmanSessionIntegration) string {
							return sessions[i].ErrorToString()
						}, ContainSubstring("failed to find image"),
					))
				} else {
					// check os and architesture.
					matchers = append(matchers, WithTransform(
						func(sessions []*PodmanSessionIntegration) string {
							return sessions[i].OutputToString()
						}, Equal(opSys+"/"+arch),
					))
				}
			}
			return And(matchers...)
		}
		/*##########################################################################################################*/

		var bldPlatformJSON = func(platformStr string) ([]byte, error) {

			// Transforms the provided string into the json expected
			// to describe a platform.

			// Care!. A platform may not actually have a variant
			// element
			type platform2 struct {
				Architecture string `json:"architecture"`
				Os           string `json:"os"`
			}

			type platform3 struct {
				platform2
				Variant string `json:"variant"`
			}

			var fields = strings.Split(platformStr, "/")

			switch len(fields) {
			case 2:
				var platform platform2
				platform.Os = fields[0]
				platform.Architecture = fields[1]
				return json.Marshal(platform)
			case 3:
				var platform platform3
				platform.Os = fields[0]
				platform.Architecture = fields[1]
				platform.Variant = fields[2]
				return json.Marshal(platform)
			default:
				return []byte{}, errors.New("Could not build JSON to represent this platform")
			}
		}
		/*##########################################################################################################*/

		var indicateJSONthatConformsToSpecification = func(bldSpecs []build) types.GomegaMatcher {
			// Checks that the JSON describing the manifest matches our expectation.
			// It confirms:
			//  * the number of images present is as expected,
			//  * the architecture of the different images present is as expected.
			// This can be use to check json returned by both manifest inspect
			// and skopeo

			// Assemble all our expected json into a single string.
			compositeJson := "["
			for i, bldSpec := range bldSpecs {
				json, err := bldPlatformJSON(bldSpec.arch)
				if err != nil {
					fmt.Printf("err = %s\n", err)
				} else {
					switch i {
					case 0:
						compositeJson += string(json)
					default:
						compositeJson += ("," + string(json))
					}
				}
			}
			compositeJson += ("]")

			return And(
				WithTransform(func(session *PodmanSessionIntegration) string {
					// Confirms that the json describing the architectures in
					// the build is as we expect.
					x, _ := session.jq("[.manifests[].platform] | sort_by(.os, .architecture, .variant)")
					return x
				}, MatchJSON(compositeJson)),

				WithTransform(func(session *PodmanSessionIntegration) string {
					// Confirms that the number of images in the json matches
					// what is expected as indicated by the length of the bldSpecs
					// array.
					x, _ := session.jq(".manifests | length")
					return x
				}, Equal(fmt.Sprintf("%d", len(bldSpecs)))),
			)
		}
		/*##########################################################################################################*/

		var transformScenario = func(scenario withTestScenarioOf) withTestScenarioOf {

			scenario.params = strings.ReplaceAll(scenario.params, "HOST_ARCH", hostArch)

			if scenario.tag == GOOD_SHORT_TAG {
				scenario.tag = goodTagBase + strings.ToLower(RandomString(10))
			}

			if scenario.tag == GOOD_LONG_TAG {
				scenario.tag = goodTagBase + strings.ToLower(RandomString(10)) + ":tag"
			}

			if scenario.tag == GOOD_VERY_LONG_TAG {
				scenario.tag = goodTagBase + strings.ToLower(RandomString(10)) + "/path/path/path:tag"
			}

			return scenario
		}
		/*##########################################################################################################*/

		var transformExpectedBuild = func(scenario withTestScenarioOf, builds expectBuildsOf) (withTestScenarioOf, expectBuildsOf) {
			// The values passed into the test need to be massaged a little before we can use them.
			//
			// The only parameters that we can inject DIRECTLY into the table are ones that are
			// initialised during ginkgo's 'preparation' phase. (i.e. values set BEFORE any tests
			// start running.)
			//
			// However we can get around this to some extent, by replacing the value passed into the table
			// with a variable that WAS configured at runtime.
			//
			// In particular, the value of HOST_ARCH is not known until the test is actually running.

			scenario = transformScenario(scenario)

			for i, build := range builds {
				if build.arch == HOST_ARCH {
					builds[i].arch = hostArch
				}
			}

			// We need the rows in our expected build to be sorted so that they reflect the
			// order they will be present in any json extracted during the tests.
			slices.SortStableFunc(builds,
				func(a, b build) int {
					return strings.Compare(a.arch, b.arch)
				})

			return scenario, builds
		}
		/*##########################################################################################################*/

		var prepareParameterArray = func(farm string, params string) []string {
			// Turn the farm and, parameter string as passed into a properly
			// formatted set of strings.

			farm = strings.Trim(farm, " ")
			parameters := []string{}

			if farm != "" {
				parameters = []string{"--farm", farm}
			}

			remainder := strings.Trim(params, " ")

			for _, str := range strings.Split(remainder, " ") {
				str := strings.Trim(str, " ")
				if str != "" {
					parameters = append(parameters, str)
				}
			}
			return parameters
		}
		/*##########################################################################################################*/

		var performFarmBuildAccordingToScenario = func(podmanTestInteg *PodmanTestIntegration, scenario withTestScenarioOf) *Sessions {

			// Function will execute a farm build according to the particular test scenario
			// passed to it, and then run some secondary utilities to analyse the outcome of
			// the run. (skopeo output, manaifest checks, image checks)

			// The output of all these is passed back so that the success/failure of the test
			// can be established.

			var sessions Sessions

			// "--farm", scenario.farm,
			buildCmd := slices.Concat([]string{
				"farm", "build",
				"--tls-verify=false"},
				prepareParameterArray(scenario.farm, scenario.params),
				[]string{
					"--tag", scenario.tag,
					filepath.Join(contextsDir, scenario.image.contextDir)},
			)
			verifyCmd := []string{"manifest", "inspect", "--tls-verify=false", scenario.tag}

			// Run the build
			sessions.Bld = podmanTestInteg.Podman(buildCmd)
			sessions.Bld.WaitWithDefaultTimeout()

			// Run the manifest check
			sessions.Manifest = podmanTestInteg.Podman(verifyCmd)
			sessions.Manifest.WaitWithDefaultTimeout()

			// skopeo checks
			sessions.Skopeo = skopeo(scenario.tag)
			sessions.Skopeo.WaitWithDefaultTimeout()

			return &sessions
		}
		/*##########################################################################################################*/

		var verifyAnyRetainedImages = func(podmanTestInteg *PodmanTestIntegration, bldSession *PodmanSessionIntegration, expectedBuilds expectBuildsOf) ([]*PodmanSessionIntegration, error) {
			// Here, we check that the images themselves have been built as they should have been.
			// Note: we can only do this if the images have been retained (--clean=false)

			var imgSessions = []*PodmanSessionIntegration{}

			// For each build we expect to have been made, we scan the output from the "farm build"
			// in order to identify the Ids of the images that have been built.
			for _, build := range expectedBuilds {
				imageId, err := scanSessionForBuiltImageId(bldSession, build)

				if err != nil {
					// Scraping image Id from the programs output may be less
					// than 100% reliable. So bail if we can't the ID
					return []*PodmanSessionIntegration{}, err
				}

				var imgSess *PodmanSessionIntegration = nil

				if build.expectedTobeBuiltOn == LOCAL_HOST {
					imgSess = podmanTestInteg.Podman([]string{"image", "inspect", "--format", "{{.Os}}/{{.Architecture}}", imageId})
				} else {
					imgSess = podmanTest.Podman([]string{"--remote", "--connection", build.expectedTobeBuiltOn, "image", "inspect", "--format", "{{.Os}}/{{.Architecture}}", imageId})
				}
				imgSess.WaitWithDefaultTimeout()

				imgSessions = append(imgSessions, imgSess)
			}

			return imgSessions, nil
		}
		/*##########################################################################################################*/

		BeforeEach(func() {
			// Skip("Skipping All.")
			// CRITICALLY IMPORTANT TO SET THE ENV, OTHERWISE AND PODMAN
			// WON'T PICK UP THE DEFINED CONNECTIONS!
			os.Setenv("PODMAN_CONNECTIONS_CONF", connectionsConf)
			os.Setenv("CONTAINERS_CONF", containersConf)

			// Need a fresh clean registtry for each test.
			registryName = runRegistry()

			// podmanlocal will be used for performing local tests (ie it will use the podman binary).
			// The standard podmanTest structure automatically created by the
			// standard test environment will perform the remote tests (i.e. using the podman-remote binary).
			podmanLocal = NewPodmanLocal(GlobalTmpDir)

		})
		/*##########################################################################################################*/

		AfterEach(func() {

			// NB Registry Entries are not preserved on restart.
			stopRegistry(registryName)

			// The standard environment doesn't know anything about podmanLocal so
			// we need to clean it up ourselves.

			podmanLocal.cleanup()
		})
		/*##########################################################################################################*/

		DescribeTableSubtree("Basic builds        :",

			// We are using additional podman server processes as proxies for actual remote servers. The testing
			// environment only provides that functionality when the ginkgo test executable is built using the
			// 'remoteintegration' target.  An unwanted side effect of that is that the podmanTest structure
			// that is usually used to run the test gets configured to run the podman-remote binary.
			//
			// Since we want tests to be run with both podman and podman-remote binaries, a custom PodmanLocal
			// structure has been introduced. This is essentially the form podmanTest would have taken if is had
			// built under the `localintegration` target
			//
			// So:
			//	 * podmanTest.Podman(<command>) ---> podman-remote --remote --url unix:///run... <command>
			//	 * podmanLocal.Podman(<command>) ---> podman  <command>

			func(scenario withTestScenarioOf, expectedBuilds expectBuildsOf) {

				var successfulOperationTest = func(
					podmanTestInteg *PodmanTestIntegration,
					scenario withTestScenarioOf,
					expectedBuilds expectBuildsOf) {

					// Note: The test may be performed with either the a local
					// or remote variant of podmanTest, so to make it explicit,
					// we are passing that as a parameter rather than just
					// accessing the global podmanTest (though we could)

					var sessions *Sessions

					// Sorting etc
					scenario, expectedBuilds = transformExpectedBuild(scenario, expectedBuilds)

					// Run the build and capture the outputs for analysis
					sessions = performFarmBuildAccordingToScenario(podmanTestInteg, scenario)

					// Providing the build has not elected to remove them (--clean), there
					// ought to be a series of untagged images on the node. We need to scan
					// these for validation.
					imgSessions, retainedImageErr := verifyAnyRetainedImages(podmanTestInteg, sessions.Bld, expectedBuilds)

					// We are expecting all these build to succeed, so first check
					// that nothing is reporting any failures.
					By("Confirm that the farm build has completed without errors")
					Expect(sessions).To(indicateARunWithoutErrorsReported(scenario.tag), "Failed to indicateARunWithoutErrorsReported()")

					// Now check that the farm has built all images we were expecting to
					// see, on the connections we were expecting to see them.
					By("Confirm that the build has built what it was supposed to")
					Expect(sessions.Bld).To(indicateAFarmBuiltToSpecification(expectedBuilds), "Failed to indicateAFarmBuiltToSpecification()")

					// Likewise confirm the manifest both locally and remotely containi
					// the images we expect to see, and nothing else.
					By("Confirm that the manifest JSON looks like it was supposed to")
					Expect(sessions.Manifest).To(indicateJSONthatConformsToSpecification(expectedBuilds), "Failed to indicateJSONthatConformsToSpecification()")

					By("Confirm that the skopeo JSON looks like it was supposed to")
					Expect(sessions.Skopeo).To(indicateJSONthatConformsToSpecification(expectedBuilds), "Failed to indicateJSONthatConformsToSpecification()")

					// If we haven't been able to successfully scrape the image id from the farm
					// build output then we can't perform the following test.
					if retainedImageErr != nil {
						Skip(fmt.Sprintf("Could not identify the built image ids (Error: %s)", retainedImageErr))
					}
					By("Confirm that all non-cleaned up images can found and are of the right architecture")
					Expect(imgSessions).To(indicateAllImagesBuiltToSpecification(expectedBuilds), "Failed to indicateAllImagesBuiltToSpecification()")
				}

				/*##########################################################################################################*/
				/*##########################################################################################################*/

				It("using podman binary", func() {
					successfulOperationTest(podmanLocal.PodmanTestIntegration, scenario, expectedBuilds)
				})
				/*##########################################################################################################*/

				It("using podman-remote binary", func() {
					successfulOperationTest(podmanTest, scenario, expectedBuilds)
				})
				/*##########################################################################################################*/
			},
			//
			//
			/* #############################################################################################################*/
			/* #############################################################################################################*/
			/*                                                                                                              */
			/*  Farm Build Test Scearios : scenarios expected to succeed                                                    */
			/*                                                                                                              */
			/* #############################################################################################################*/
			Entry("proxyFarm build with default parameters",
				withTestScenarioOf{farm: "proxyFarm", params: "", image: standardTestImage, tag: GOOD_SHORT_TAG},
				expectBuildsOf{
					build{arch: HOST_ARCH, expectedTobeBuiltOn: LOCAL_HOST, usingEmulation: false, withCleanup: false},
				},
			),
			//
			Entry("proxyFarm build with full reference form of tag (registry:port/path:tag)",
				withTestScenarioOf{farm: "proxyFarm", params: "", image: standardTestImage, tag: GOOD_LONG_TAG},
				expectBuildsOf{
					build{arch: HOST_ARCH, expectedTobeBuiltOn: LOCAL_HOST, usingEmulation: false, withCleanup: false},
				},
			),
			//
			Entry("proxyFarm build with even fuller reference form of tag (registry:port/path/path/path:tag)",
				withTestScenarioOf{farm: "proxyFarm", params: "", image: standardTestImage, tag: GOOD_VERY_LONG_TAG},
				expectBuildsOf{
					build{arch: HOST_ARCH, expectedTobeBuiltOn: LOCAL_HOST, usingEmulation: false, withCleanup: false},
				},
			),
			//
			Entry("proxyFarm build with --local=true",
				withTestScenarioOf{farm: "proxyFarm", params: "--local=true", image: standardTestImage, tag: GOOD_SHORT_TAG},
				expectBuildsOf{
					build{arch: HOST_ARCH, expectedTobeBuiltOn: LOCAL_HOST, usingEmulation: false, withCleanup: false},
				},
			),
			//
			Entry("proxyFarm build with --local=false",
				withTestScenarioOf{farm: "proxyFarm", params: "--local=false", image: standardTestImage, tag: GOOD_SHORT_TAG},
				expectBuildsOf{
					build{arch: HOST_ARCH, expectedTobeBuiltOn: "ConA", usingEmulation: false, withCleanup: false},
				},
			),
			//
			Entry("proxyFarm build with --cleanup=false",
				withTestScenarioOf{farm: "proxyFarm", params: "--local=false --cleanup=false", image: standardTestImage, tag: GOOD_SHORT_TAG},
				expectBuildsOf{
					build{arch: HOST_ARCH, expectedTobeBuiltOn: "ConA", usingEmulation: false, withCleanup: false},
				},
			),
			//
			Entry("proxyFarm build with --cleanup=true",
				withTestScenarioOf{farm: "proxyFarm", params: "--local=false --cleanup=true", image: standardTestImage, tag: GOOD_SHORT_TAG},
				expectBuildsOf{
					build{arch: HOST_ARCH, expectedTobeBuiltOn: "ConA", usingEmulation: false, withCleanup: true},
				},
			),
			//
			Entry("proxyFarm build with --platforms= empty string",
				withTestScenarioOf{farm: "proxyFarm", params: "--local=false --platforms=", image: standardTestImage, tag: GOOD_SHORT_TAG},
				expectBuildsOf{
					build{arch: HOST_ARCH, expectedTobeBuiltOn: "ConA", usingEmulation: false, withCleanup: false},
				},
			),
			//
			Entry("proxyFarm build with --platforms= HOST_ARCH",
				withTestScenarioOf{farm: "proxyFarm", params: "--local=false --platforms=HOST_ARCH", image: standardTestImage, tag: GOOD_SHORT_TAG},
				expectBuildsOf{
					build{arch: HOST_ARCH, expectedTobeBuiltOn: "ConA", usingEmulation: false, withCleanup: false},
				},
			),
			//
			Entry("No farm specified (Default Farm)",
				withTestScenarioOf{farm: "", params: "--local=false", image: standardTestImage, tag: GOOD_SHORT_TAG},
				expectBuildsOf{
					build{arch: HOST_ARCH, expectedTobeBuiltOn: "Default", usingEmulation: false, withCleanup: false},
				},
			),
			Entry("Empty Farm but with a local builder to fall back to.",
				withTestScenarioOf{farm: "emptyFarm", params: "", image: standardTestImage, tag: GOOD_SHORT_TAG},
				expectBuildsOf{
					build{arch: HOST_ARCH, expectedTobeBuiltOn: LOCAL_HOST, usingEmulation: false, withCleanup: false},
				},
			),
			//
			Entry("Multi Node Farm but with a single distinct architecture available on it",
				withTestScenarioOf{farm: "multinodeFarm", params: "--local=true", image: standardTestImage, tag: GOOD_SHORT_TAG},
				expectBuildsOf{
					build{arch: HOST_ARCH, expectedTobeBuiltOn: LOCAL_HOST, usingEmulation: false, withCleanup: false},
				},
			),
		)
		/*##########################################################################################################*/
		/*##########################################################################################################*/

		DescribeTableSubtree("Failure Scenarios        :",

			func(scenario withTestScenarioOf, expectedFailure expectFailureWith) {

				var failedOperationTest = func(
					podmanTestInteg *PodmanTestIntegration,
					scenario withTestScenarioOf,
					expectedFailure expectFailureWith) {

					// Note: The test may be performed with either the a local
					// or remote variant of podmanTest, so to make it explicit,
					// we are passing that as a parameter rather than just
					// accessing the global podmanTest (though we could)

					// Substitute any macros in use
					scenario = transformScenario(scenario)

					// Run the build and capture the outputs for analysis
					sessions := performFarmBuildAccordingToScenario(podmanTestInteg, scenario)

					// We are expecting all these build to succeed, so first check
					// that nothing is reporting any failures.
					By("Check whether the farm build is reporting errors")
					Expect(sessions.Bld).NotTo(indicateAnErrorFreeFarmBuild(scenario.tag), "indicateAnErrorFreeFarmBuild() unexpectedly succeeded")

					// Now check whether the error message is as expected
					Expect(sessions.Bld.ErrorToString()).Should(ContainSubstring(expectedFailure.message), "Unexpected Error message")
				}

				/*##########################################################################################################*/

				It("using podman binary", func() {
					failedOperationTest(podmanLocal.PodmanTestIntegration, scenario, expectedFailure)
				})
				/*##########################################################################################################*/

				It("using podman-remote binary", func() {
					failedOperationTest(podmanTest, scenario, expectedFailure)
				})
				/*##########################################################################################################*/
			},
			//
			/* #############################################################################################################*/
			/*                                                                                                              */
			/*  Farm Build Test Scearios : scenarios expected to fail.                                                      */
			/*                                                                                                              */
			/* #############################################################################################################*/
			//
			Entry("Empty Farm and no local builder to fall back to.",
				withTestScenarioOf{farm: "emptyFarm", params: "--local=false", image: standardTestImage, tag: GOOD_SHORT_TAG},
				expectFailureWith{message: "no builders configured"},
			),
			//
			Entry("proxyFarm build with --platforms= HOST_ARCH + any other",
				withTestScenarioOf{farm: "proxyFarm", params: "--local=false --platforms=HOST_ARCH,linux/unknown", image: standardTestImage, tag: GOOD_SHORT_TAG},
				expectFailureWith{message: "no builder capable of building for platform"},
			),
			//
			Entry("proxyFarm build with --platforms=unknown",
				withTestScenarioOf{farm: "proxyFarm", params: "--local=false --platforms=linux/unknown", image: standardTestImage, tag: GOOD_SHORT_TAG},
				expectFailureWith{message: "no builder capable of building for platform"},
			),
			//
			Entry("proxyFarm build with registry name missing from tag",
				withTestScenarioOf{farm: "proxyFarm", params: "", image: standardTestImage, tag: "name-only"},
				expectFailureWith{message: "not a full image reference name"},
			),
			//
			Entry("proxyFarm build with incompletely formed registry ",
				withTestScenarioOf{farm: "proxyFarm", params: "", image: standardTestImage, tag: "/name-only"},
				expectFailureWith{message: "invalid reference format"},
			),
			//
			Entry("Non-existent Farm",
				withTestScenarioOf{farm: "nonExistentFarm", params: "", image: standardTestImage, tag: GOOD_SHORT_TAG},
				expectFailureWith{message: "farm \"nonExistentFarm\" not found"},
			),
			//
			Entry("Farm with one of the nodes offline",
				withTestScenarioOf{farm: "offlineFarm", params: "", image: standardTestImage, tag: GOOD_SHORT_TAG},
				expectFailureWith{message: "unable to connect to Podman socket"},
			),
		)
		/*##########################################################################################################*/
	})
})
