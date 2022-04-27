//go:build benchmarks
// +build benchmarks

package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	podmanRegistry "github.com/containers/podman/v4/hack/podman-registry-go"
	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"github.com/sirupsen/logrus"
)

var (
	// Number of times to execute each benchmark.
	numBenchmarkSamples = 3
	// All benchmarks are ququed here.
	allBenchmarks []benchmark
)

// An internal struct for queuing benchmarks.
type benchmark struct {
	// The name of the benchmark.
	name string
	// The function to execute.
	main func()
	// Allows for extending a benchmark.
	options newBenchmarkOptions
}

// Allows for customizing the benchnmark in an easy to extend way.
type newBenchmarkOptions struct {
	// Sets the benchmark's init function.
	init func()
	// Run a local registry for this benchmark.
	needsRegistry bool
}

// Queue a new benchmark.
func newBenchmark(name string, main func(), options *newBenchmarkOptions) {
	bm := benchmark{name: name, main: main}
	if options != nil {
		bm.options = *options
	}
	allBenchmarks = append(allBenchmarks, bm)
}

var _ = Describe("Podman Benchmark Suite", func() {
	var (
		timedir    string
		podmanTest *PodmanTestIntegration
	)

	setup := func() {
		tempdir, err := CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
		timedir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
	}

	cleanup := func() {
		podmanTest.Cleanup()
		os.RemoveAll(timedir)
	}

	totalMemoryInKb := func() (total uint64) {
		files, err := ioutil.ReadDir(timedir)
		if err != nil {
			Fail(fmt.Sprintf("Error reading timing dir: %v", err))
		}

		for _, f := range files {
			if f.IsDir() {
				continue
			}
			raw, err := ioutil.ReadFile(path.Join(timedir, f.Name()))
			if err != nil {
				Fail(fmt.Sprintf("Error reading timing file: %v", err))
			}
			rawS := strings.TrimSuffix(string(raw), "\n")
			number, err := strconv.ParseUint(rawS, 10, 64)
			if err != nil {
				Fail(fmt.Sprintf("Error converting timing file to numeric value: %v", err))
			}
			total += number
		}

		return total
	}

	// Make sure to clean up after the benchmarks.
	AfterEach(func() {
		cleanup()
	})

	// All benchmarks are executed here to have *one* table listing all data.
	Measure("Podman Benchmark Suite", func(b Benchmarker) {
		for i := range allBenchmarks {
			setup()
			bm := allBenchmarks[i]

			// Start a local registry if requested.
			var registry *podmanRegistry.Registry
			if bm.options.needsRegistry {
				reg, err := podmanRegistry.Start()
				if err != nil {
					logrus.Errorf("Error starting registry: %v", err)
					os.Exit(1)
				}
				registry = reg
				os.Setenv(podmanRegistry.UserKey, reg.User)
				os.Setenv(podmanRegistry.PassKey, reg.Password)
				os.Setenv(podmanRegistry.PortKey, reg.Port)
			}

			if bm.options.init != nil {
				bm.options.init()
			}

			// Set the time dir only for the main() function.
			os.Setenv(EnvTimeDir, timedir)
			b.Time("[CPU] "+bm.name, bm.main)
			os.Unsetenv(EnvTimeDir)

			mem := totalMemoryInKb()
			b.RecordValueWithPrecision("[MEM] "+bm.name, float64(mem), "KB", 1)

			// Stop the local registry.
			if bm.options.needsRegistry {
				os.Unsetenv(podmanRegistry.UserKey)
				os.Unsetenv(podmanRegistry.PassKey)
				os.Unsetenv(podmanRegistry.PortKey)
				if err := registry.Stop(); err != nil {
					logrus.Errorf("Error stopping registry: %v", err)
					os.Exit(1)
				}
			}

			cleanup()
		}
	}, numBenchmarkSamples)

	BeforeEach(func() {

		// --------------------------------------------------------------------------
		// IMAGE BENCHMARKS
		// --------------------------------------------------------------------------

		newBenchmark("podman images", func() {
			session := podmanTest.Podman([]string{"images"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
		}, nil)

		newBenchmark("podman pull", func() {
			session := podmanTest.Podman([]string{"pull", "quay.io/libpod/cirros"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
		}, nil)

		newBenchmark("podman load [docker]", func() {
			session := podmanTest.Podman([]string{"load", "-i", "./testdata/docker-two-images.tar.xz"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
		}, nil)

		newBenchmark("podman load [oci]", func() {
			session := podmanTest.Podman([]string{"load", "-i", "./testdata/oci-registry-name.tar.gz"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
		}, nil)

		newBenchmark("podman save", func() {
			session := podmanTest.Podman([]string{"save", ALPINE, "-o", path.Join(podmanTest.TempDir, "alpine.tar")})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
		}, nil)

		newBenchmark("podman image inspect", func() {
			session := podmanTest.Podman([]string{"inspect", ALPINE})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
		}, nil)

		newBenchmark("podman login + logout", func() {
			user := os.Getenv(podmanRegistry.UserKey)
			pass := os.Getenv(podmanRegistry.PassKey)
			port := os.Getenv(podmanRegistry.PortKey)

			session := podmanTest.Podman([]string{"login", "-u", user, "-p", pass, "--tls-verify=false", "localhost:" + port})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))

			session = podmanTest.Podman([]string{"logout", "localhost:" + port})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
		}, &newBenchmarkOptions{needsRegistry: true})

		// --------------------------------------------------------------------------
		// CONTAINER BENCHMARKS
		// --------------------------------------------------------------------------

		newBenchmark("podman create", func() {
			session := podmanTest.Podman([]string{"run", ALPINE, "true"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
		}, nil)

		newBenchmark("podman start", func() {
			session := podmanTest.Podman([]string{"start", "foo"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
		}, &newBenchmarkOptions{
			init: func() {
				session := podmanTest.Podman([]string{"create", "--name=foo", ALPINE, "true"})
				session.WaitWithDefaultTimeout()
				Expect(session).Should(Exit(0))
			},
		})

		newBenchmark("podman run", func() {
			session := podmanTest.Podman([]string{"run", ALPINE, "true"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(Exit(0))
		}, nil)
	})
})
