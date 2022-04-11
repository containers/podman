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

	. "github.com/containers/podman/v4/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
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
	// Function is run before `main`.
	init func()
}

// Allows for customizing the benchnmark in an easy to extend way.
type newBenchmarkOptions struct {
	// Sets the benchmark's init function.
	init func()
}

// Queue a new benchmark.
func newBenchmark(name string, main func(), options *newBenchmarkOptions) {
	bm := benchmark{name: name, main: main}
	if options != nil {
		bm.init = options.init
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
			if bm.init != nil {
				bm.init()
			}

			// Set the time dir only for the main() function.
			os.Setenv(EnvTimeDir, timedir)
			b.Time("[CPU] "+bm.name, bm.main)
			os.Unsetenv(EnvTimeDir)

			mem := totalMemoryInKb()
			b.RecordValueWithPrecision("[MEM] "+bm.name, float64(mem), "KB", 1)
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
