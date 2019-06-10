package integration_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/types"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Running Specs", func() {
	var pathToTest string

	isWindows := (runtime.GOOS == "windows")
	denoter := "•"

	if isWindows {
		denoter = "+"
	}

	Context("when pointed at the current directory", func() {
		BeforeEach(func() {
			pathToTest = tmpPath("ginkgo")
			copyIn(fixturePath("passing_ginkgo_tests"), pathToTest, false)
		})

		It("should run the tests in the working directory", func() {
			session := startGinkgo(pathToTest, "--noColor")
			Eventually(session).Should(gexec.Exit(0))
			output := string(session.Out.Contents())

			Ω(output).Should(ContainSubstring("Running Suite: Passing_ginkgo_tests Suite"))
			Ω(output).Should(ContainSubstring(strings.Repeat(denoter, 4)))
			Ω(output).Should(ContainSubstring("SUCCESS! -- 4 Passed"))
			Ω(output).Should(ContainSubstring("Test Suite Passed"))
		})
	})

	Context("when passed an explicit package to run", func() {
		BeforeEach(func() {
			pathToTest = tmpPath("ginkgo")
			copyIn(fixturePath("passing_ginkgo_tests"), pathToTest, false)
		})

		It("should run the ginkgo style tests", func() {
			session := startGinkgo(tmpDir, "--noColor", pathToTest)
			Eventually(session).Should(gexec.Exit(0))
			output := string(session.Out.Contents())

			Ω(output).Should(ContainSubstring("Running Suite: Passing_ginkgo_tests Suite"))
			Ω(output).Should(ContainSubstring(strings.Repeat(denoter, 4)))
			Ω(output).Should(ContainSubstring("SUCCESS! -- 4 Passed"))
			Ω(output).Should(ContainSubstring("Test Suite Passed"))
		})
	})

	Context("when passed a number of packages to run", func() {
		BeforeEach(func() {
			pathToTest = tmpPath("ginkgo")
			otherPathToTest := tmpPath("other")
			copyIn(fixturePath("passing_ginkgo_tests"), pathToTest, false)
			copyIn(fixturePath("more_ginkgo_tests"), otherPathToTest, false)
		})

		It("should run the ginkgo style tests", func() {
			session := startGinkgo(tmpDir, "--noColor", "--succinct=false", "ginkgo", "./other")
			Eventually(session).Should(gexec.Exit(0))
			output := string(session.Out.Contents())

			Ω(output).Should(ContainSubstring("Running Suite: Passing_ginkgo_tests Suite"))
			Ω(output).Should(ContainSubstring("Running Suite: More_ginkgo_tests Suite"))
			Ω(output).Should(ContainSubstring("Test Suite Passed"))
		})
	})

	Context("when passed a number of packages to run, some of which have focused tests", func() {
		BeforeEach(func() {
			pathToTest = tmpPath("ginkgo")
			otherPathToTest := tmpPath("other")
			focusedPathToTest := tmpPath("focused")
			copyIn(fixturePath("passing_ginkgo_tests"), pathToTest, false)
			copyIn(fixturePath("more_ginkgo_tests"), otherPathToTest, false)
			copyIn(fixturePath("focused_fixture"), focusedPathToTest, false)
		})

		It("should exit with a status code of 2 and explain why", func() {
			session := startGinkgo(tmpDir, "--noColor", "--succinct=false", "-r")
			Eventually(session).Should(gexec.Exit(types.GINKGO_FOCUS_EXIT_CODE))
			output := string(session.Out.Contents())

			Ω(output).Should(ContainSubstring("Running Suite: Passing_ginkgo_tests Suite"))
			Ω(output).Should(ContainSubstring("Running Suite: More_ginkgo_tests Suite"))
			Ω(output).Should(ContainSubstring("Test Suite Passed"))
			Ω(output).Should(ContainSubstring("Detected Programmatic Focus - setting exit status to %d", types.GINKGO_FOCUS_EXIT_CODE))
		})

		Context("when the GINKGO_EDITOR_INTEGRATION environment variable is set", func() {
			BeforeEach(func() {
				os.Setenv("GINKGO_EDITOR_INTEGRATION", "true")
			})
			AfterEach(func() {
				os.Setenv("GINKGO_EDITOR_INTEGRATION", "")
			})
			It("should exit with a status code of 0 to allow a coverage file to be generated", func() {
				session := startGinkgo(tmpDir, "--noColor", "--succinct=false", "-r")
				Eventually(session).Should(gexec.Exit(0))
				output := string(session.Out.Contents())

				Ω(output).Should(ContainSubstring("Running Suite: Passing_ginkgo_tests Suite"))
				Ω(output).Should(ContainSubstring("Running Suite: More_ginkgo_tests Suite"))
				Ω(output).Should(ContainSubstring("Test Suite Passed"))
			})
		})
	})

	Context("when told to skipPackages", func() {
		BeforeEach(func() {
			pathToTest = tmpPath("ginkgo")
			otherPathToTest := tmpPath("other")
			focusedPathToTest := tmpPath("focused")
			copyIn(fixturePath("passing_ginkgo_tests"), pathToTest, false)
			copyIn(fixturePath("more_ginkgo_tests"), otherPathToTest, false)
			copyIn(fixturePath("focused_fixture"), focusedPathToTest, false)
		})

		It("should skip packages that match the list", func() {
			session := startGinkgo(tmpDir, "--noColor", "--skipPackage=other,focused", "-r")
			Eventually(session).Should(gexec.Exit(0))
			output := string(session.Out.Contents())

			Ω(output).Should(ContainSubstring("Passing_ginkgo_tests Suite"))
			Ω(output).ShouldNot(ContainSubstring("More_ginkgo_tests Suite"))
			Ω(output).ShouldNot(ContainSubstring("Focused_fixture Suite"))
			Ω(output).Should(ContainSubstring("Test Suite Passed"))
		})

		Context("when all packages are skipped", func() {
			It("should not run anything, but still exit 0", func() {
				session := startGinkgo(tmpDir, "--noColor", "--skipPackage=other,focused,ginkgo", "-r")
				Eventually(session).Should(gexec.Exit(0))
				output := string(session.Out.Contents())

				Ω(output).Should(ContainSubstring("All tests skipped!"))
				Ω(output).ShouldNot(ContainSubstring("Passing_ginkgo_tests Suite"))
				Ω(output).ShouldNot(ContainSubstring("More_ginkgo_tests Suite"))
				Ω(output).ShouldNot(ContainSubstring("Focused_fixture Suite"))
				Ω(output).ShouldNot(ContainSubstring("Test Suite Passed"))
			})
		})
	})

	Context("when there are no tests to run", func() {
		It("should exit 1", func() {
			session := startGinkgo(tmpDir, "--noColor", "--skipPackage=other,focused", "-r")
			Eventually(session).Should(gexec.Exit(1))
			output := string(session.Err.Contents())

			Ω(output).Should(ContainSubstring("Found no test suites"))
		})
	})

	Context("when there are test files but `go test` reports there are no tests to run", func() {
		BeforeEach(func() {
			pathToTest = tmpPath("ginkgo")
			copyIn(fixturePath("no_test_fn"), pathToTest, false)
		})

		It("suggests running ginkgo bootstrap", func() {
			session := startGinkgo(tmpDir, "--noColor", "--skipPackage=other,focused", "-r")
			Eventually(session).Should(gexec.Exit(0))
			output := string(session.Err.Contents())

			Ω(output).Should(ContainSubstring(`Found no test suites, did you forget to run "ginkgo bootstrap"?`))
		})

		It("fails if told to requireSuite", func() {
			session := startGinkgo(tmpDir, "--noColor", "--skipPackage=other,focused", "-r", "-requireSuite")
			Eventually(session).Should(gexec.Exit(1))
			output := string(session.Err.Contents())

			Ω(output).Should(ContainSubstring(`Found no test suites, did you forget to run "ginkgo bootstrap"?`))
		})
	})

	Context("when told to randomizeSuites", func() {
		BeforeEach(func() {
			pathToTest = tmpPath("ginkgo")
			otherPathToTest := tmpPath("other")
			copyIn(fixturePath("passing_ginkgo_tests"), pathToTest, false)
			copyIn(fixturePath("more_ginkgo_tests"), otherPathToTest, false)
		})

		It("should skip packages that match the regexp", func() {
			session := startGinkgo(tmpDir, "--noColor", "--randomizeSuites", "-r", "--seed=2")
			Eventually(session).Should(gexec.Exit(0))

			Ω(session).Should(gbytes.Say("More_ginkgo_tests Suite"))
			Ω(session).Should(gbytes.Say("Passing_ginkgo_tests Suite"))

			session = startGinkgo(tmpDir, "--noColor", "--randomizeSuites", "-r", "--seed=3")
			Eventually(session).Should(gexec.Exit(0))

			Ω(session).Should(gbytes.Say("Passing_ginkgo_tests Suite"))
			Ω(session).Should(gbytes.Say("More_ginkgo_tests Suite"))
		})
	})

	Context("when pointed at a package with xunit style tests", func() {
		BeforeEach(func() {
			pathToTest = tmpPath("xunit")
			copyIn(fixturePath("xunit_tests"), pathToTest, false)
		})

		It("should run the xunit style tests", func() {
			session := startGinkgo(pathToTest)
			Eventually(session).Should(gexec.Exit(0))
			output := string(session.Out.Contents())

			Ω(output).Should(ContainSubstring("--- PASS: TestAlwaysTrue"))
			Ω(output).Should(ContainSubstring("Test Suite Passed"))
		})
	})

	Context("when pointed at a package with no tests", func() {
		BeforeEach(func() {
			pathToTest = tmpPath("no_tests")
			copyIn(fixturePath("no_tests"), pathToTest, false)
		})

		It("should fail", func() {
			session := startGinkgo(pathToTest, "--noColor")
			Eventually(session).Should(gexec.Exit(1))

			Ω(session.Err.Contents()).Should(ContainSubstring("Found no test suites"))
		})
	})

	Context("when pointed at a package that fails to compile", func() {
		BeforeEach(func() {
			pathToTest = tmpPath("does_not_compile")
			copyIn(fixturePath("does_not_compile"), pathToTest, false)
		})

		It("should fail", func() {
			session := startGinkgo(pathToTest, "--noColor")
			Eventually(session).Should(gexec.Exit(1))
			output := string(session.Out.Contents())

			Ω(output).Should(ContainSubstring("Failed to compile"))
		})
	})

	Context("when running in parallel", func() {
		BeforeEach(func() {
			pathToTest = tmpPath("ginkgo")
			copyIn(fixturePath("passing_ginkgo_tests"), pathToTest, false)
		})

		Context("with a specific number of -nodes", func() {
			It("should use the specified number of nodes", func() {
				session := startGinkgo(pathToTest, "--noColor", "-succinct", "-nodes=2")
				Eventually(session).Should(gexec.Exit(0))
				output := string(session.Out.Contents())

				Ω(output).Should(MatchRegexp(`\[\d+\] Passing_ginkgo_tests Suite - 4 specs - 2 nodes [%s]{4} SUCCESS! \d+(\.\d+)?[muµ]s`, regexp.QuoteMeta(denoter)))
				Ω(output).Should(ContainSubstring("Test Suite Passed"))
			})
		})

		Context("with -p", func() {
			It("it should autocompute the number of nodes", func() {
				session := startGinkgo(pathToTest, "--noColor", "-succinct", "-p")
				Eventually(session).Should(gexec.Exit(0))
				output := string(session.Out.Contents())

				nodes := runtime.NumCPU()
				if nodes == 1 {
					Skip("Can't test parallel testings with 1 CPU")
				}
				if nodes > 4 {
					nodes = nodes - 1
				}
				Ω(output).Should(MatchRegexp(`\[\d+\] Passing_ginkgo_tests Suite - 4 specs - %d nodes [%s]{4} SUCCESS! \d+(\.\d+)?[muµ]?s`, nodes, regexp.QuoteMeta(denoter)))
				Ω(output).Should(ContainSubstring("Test Suite Passed"))
			})
		})
	})

	Context("when running in parallel with -debug", func() {
		BeforeEach(func() {
			pathToTest = tmpPath("ginkgo")
			copyIn(fixturePath("debug_parallel_fixture"), pathToTest, false)
		})

		Context("without -v", func() {
			It("should emit node output to files on disk", func() {
				session := startGinkgo(pathToTest, "--nodes=2", "--debug")
				Eventually(session).Should(gexec.Exit(0))

				f0, err := ioutil.ReadFile(pathToTest + "/ginkgo-node-1.log")
				Ω(err).ShouldNot(HaveOccurred())
				f1, err := ioutil.ReadFile(pathToTest + "/ginkgo-node-2.log")
				Ω(err).ShouldNot(HaveOccurred())
				content := string(append(f0, f1...))

				for i := 0; i < 10; i += 1 {
					Ω(content).Should(ContainSubstring("StdOut %d\n", i))
					Ω(content).Should(ContainSubstring("GinkgoWriter %d\n", i))
				}
			})
		})

		Context("without -v", func() {
			It("should emit node output to files on disk, without duplicating the GinkgoWriter output", func() {
				session := startGinkgo(pathToTest, "--nodes=2", "--debug", "-v")
				Eventually(session).Should(gexec.Exit(0))

				f0, err := ioutil.ReadFile(pathToTest + "/ginkgo-node-1.log")
				Ω(err).ShouldNot(HaveOccurred())
				f1, err := ioutil.ReadFile(pathToTest + "/ginkgo-node-2.log")
				Ω(err).ShouldNot(HaveOccurred())
				content := string(append(f0, f1...))

				out := strings.Split(content, "GinkgoWriter 2")
				Ω(out).Should(HaveLen(2))
			})
		})
	})

	Context("when streaming in parallel", func() {
		BeforeEach(func() {
			pathToTest = tmpPath("ginkgo")
			copyIn(fixturePath("passing_ginkgo_tests"), pathToTest, false)
		})

		It("should print output in realtime", func() {
			session := startGinkgo(pathToTest, "--noColor", "-stream", "-nodes=2")
			Eventually(session).Should(gexec.Exit(0))
			output := string(session.Out.Contents())

			Ω(output).Should(ContainSubstring(`[1] Parallel test node 1/2.`))
			Ω(output).Should(ContainSubstring(`[2] Parallel test node 2/2.`))
			Ω(output).Should(ContainSubstring(`[1] SUCCESS!`))
			Ω(output).Should(ContainSubstring(`[2] SUCCESS!`))
			Ω(output).Should(ContainSubstring("Test Suite Passed"))
		})
	})

	Context("when running recursively", func() {
		BeforeEach(func() {
			passingTest := tmpPath("A")
			otherPassingTest := tmpPath("E")
			copyIn(fixturePath("passing_ginkgo_tests"), passingTest, false)
			copyIn(fixturePath("more_ginkgo_tests"), otherPassingTest, false)
		})

		Context("when all the tests pass", func() {
			Context("with the -r flag", func() {
				It("should run all the tests (in succinct mode) and succeed", func() {
					session := startGinkgo(tmpDir, "--noColor", "-r", ".")
					Eventually(session).Should(gexec.Exit(0))
					output := string(session.Out.Contents())

					outputLines := strings.Split(output, "\n")
					Ω(outputLines[0]).Should(MatchRegexp(`\[\d+\] Passing_ginkgo_tests Suite - 4/4 specs [%s]{4} SUCCESS! \d+(\.\d+)?[muµ]s PASS`, regexp.QuoteMeta(denoter)))
					Ω(outputLines[1]).Should(MatchRegexp(`\[\d+\] More_ginkgo_tests Suite - 2/2 specs [%s]{2} SUCCESS! \d+(\.\d+)?[muµ]s PASS`, regexp.QuoteMeta(denoter)))
					Ω(output).Should(ContainSubstring("Test Suite Passed"))
				})
			})
			Context("with a trailing /...", func() {
				It("should run all the tests (in succinct mode) and succeed", func() {
					session := startGinkgo(tmpDir, "--noColor", "./...")
					Eventually(session).Should(gexec.Exit(0))
					output := string(session.Out.Contents())

					outputLines := strings.Split(output, "\n")
					Ω(outputLines[0]).Should(MatchRegexp(`\[\d+\] Passing_ginkgo_tests Suite - 4/4 specs [%s]{4} SUCCESS! \d+(\.\d+)?[muµ]s PASS`, regexp.QuoteMeta(denoter)))
					Ω(outputLines[1]).Should(MatchRegexp(`\[\d+\] More_ginkgo_tests Suite - 2/2 specs [%s]{2} SUCCESS! \d+(\.\d+)?[muµ]s PASS`, regexp.QuoteMeta(denoter)))
					Ω(output).Should(ContainSubstring("Test Suite Passed"))
				})
			})
		})

		Context("when one of the packages has a failing tests", func() {
			BeforeEach(func() {
				failingTest := tmpPath("C")
				copyIn(fixturePath("failing_ginkgo_tests"), failingTest, false)
			})

			It("should fail and stop running tests", func() {
				session := startGinkgo(tmpDir, "--noColor", "-r")
				Eventually(session).Should(gexec.Exit(1))
				output := string(session.Out.Contents())

				outputLines := strings.Split(output, "\n")
				Ω(outputLines[0]).Should(MatchRegexp(`\[\d+\] Passing_ginkgo_tests Suite - 4/4 specs [%s]{4} SUCCESS! \d+(\.\d+)?[muµ]s PASS`, regexp.QuoteMeta(denoter)))
				Ω(outputLines[1]).Should(MatchRegexp(`\[\d+\] Failing_ginkgo_tests Suite - 2/2 specs`))
				Ω(output).Should(ContainSubstring(fmt.Sprintf("%s Failure", denoter)))
				Ω(output).ShouldNot(ContainSubstring("More_ginkgo_tests Suite"))
				Ω(output).Should(ContainSubstring("Test Suite Failed"))

				Ω(output).Should(ContainSubstring("Summarizing 1 Failure:"))
				Ω(output).Should(ContainSubstring("[Fail] FailingGinkgoTests [It] should fail"))
			})
		})

		Context("when one of the packages fails to compile", func() {
			BeforeEach(func() {
				doesNotCompileTest := tmpPath("C")
				copyIn(fixturePath("does_not_compile"), doesNotCompileTest, false)
			})

			It("should fail and stop running tests", func() {
				session := startGinkgo(tmpDir, "--noColor", "-r")
				Eventually(session).Should(gexec.Exit(1))
				output := string(session.Out.Contents())

				outputLines := strings.Split(output, "\n")
				Ω(outputLines[0]).Should(MatchRegexp(`\[\d+\] Passing_ginkgo_tests Suite - 4/4 specs [%s]{4} SUCCESS! \d+(\.\d+)?[muµ]s PASS`, regexp.QuoteMeta(denoter)))
				Ω(outputLines[1]).Should(ContainSubstring("Failed to compile C:"))
				Ω(output).ShouldNot(ContainSubstring("More_ginkgo_tests Suite"))
				Ω(output).Should(ContainSubstring("Test Suite Failed"))
			})
		})

		Context("when either is the case, but the keepGoing flag is set", func() {
			BeforeEach(func() {
				doesNotCompileTest := tmpPath("B")
				copyIn(fixturePath("does_not_compile"), doesNotCompileTest, false)

				failingTest := tmpPath("C")
				copyIn(fixturePath("failing_ginkgo_tests"), failingTest, false)
			})

			It("should soldier on", func() {
				session := startGinkgo(tmpDir, "--noColor", "-r", "-keepGoing")
				Eventually(session).Should(gexec.Exit(1))
				output := string(session.Out.Contents())

				outputLines := strings.Split(output, "\n")
				Ω(outputLines[0]).Should(MatchRegexp(`\[\d+\] Passing_ginkgo_tests Suite - 4/4 specs [%s]{4} SUCCESS! \d+(\.\d+)?[muµ]s PASS`, regexp.QuoteMeta(denoter)))
				Ω(outputLines[1]).Should(ContainSubstring("Failed to compile B:"))
				Ω(output).Should(MatchRegexp(`\[\d+\] Failing_ginkgo_tests Suite - 2/2 specs`))
				Ω(output).Should(ContainSubstring(fmt.Sprintf("%s Failure", denoter)))
				Ω(output).Should(MatchRegexp(`\[\d+\] More_ginkgo_tests Suite - 2/2 specs [%s]{2} SUCCESS! \d+(\.\d+)?[muµ]s PASS`, regexp.QuoteMeta(denoter)))
				Ω(output).Should(ContainSubstring("Test Suite Failed"))
			})
		})
	})

	Context("when told to keep going --untilItFails", func() {
		BeforeEach(func() {
			copyIn(fixturePath("eventually_failing"), tmpDir, false)
		})

		It("should keep rerunning the tests, until a failure occurs", func() {
			session := startGinkgo(tmpDir, "--untilItFails", "--noColor")
			Eventually(session).Should(gexec.Exit(1))
			Ω(session).Should(gbytes.Say("This was attempt #1"))
			Ω(session).Should(gbytes.Say("This was attempt #2"))
			Ω(session).Should(gbytes.Say("Tests failed on attempt #3"))

			//it should change the random seed between each test
			lines := strings.Split(string(session.Out.Contents()), "\n")
			randomSeeds := []string{}
			for _, line := range lines {
				if strings.Contains(line, "Random Seed:") {
					randomSeeds = append(randomSeeds, strings.Split(line, ": ")[1])
				}
			}
			Ω(randomSeeds[0]).ShouldNot(Equal(randomSeeds[1]))
			Ω(randomSeeds[1]).ShouldNot(Equal(randomSeeds[2]))
			Ω(randomSeeds[0]).ShouldNot(Equal(randomSeeds[2]))
		})
	})
})
