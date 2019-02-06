package integration_test

import (
	"os/exec"

	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Coverage Specs", func() {
	Context("when it runs coverage analysis in series and in parallel", func() {
		AfterEach(func() {
			removeSuccessfully("./_fixtures/coverage_fixture/coverage_fixture.coverprofile")
		})
		It("works", func() {
			session := startGinkgo("./_fixtures/coverage_fixture", "-cover")
			Eventually(session).Should(gexec.Exit(0))

			Ω(session.Out).Should(gbytes.Say(("coverage: 80.0% of statements")))

			coverFile := "./_fixtures/coverage_fixture/coverage_fixture.coverprofile"
			serialCoverProfileOutput, err := exec.Command("go", "tool", "cover", fmt.Sprintf("-func=%s", coverFile)).CombinedOutput()
			Ω(err).ShouldNot(HaveOccurred())

			removeSuccessfully(coverFile)

			Eventually(startGinkgo("./_fixtures/coverage_fixture", "-cover", "-nodes=4")).Should(gexec.Exit(0))

			parallelCoverProfileOutput, err := exec.Command("go", "tool", "cover", fmt.Sprintf("-func=%s", coverFile)).CombinedOutput()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(parallelCoverProfileOutput).Should(Equal(serialCoverProfileOutput))

			By("handling external packages", func() {
				session = startGinkgo("./_fixtures/coverage_fixture", "-coverpkg=github.com/onsi/ginkgo/integration/_fixtures/coverage_fixture,github.com/onsi/ginkgo/integration/_fixtures/coverage_fixture/external_coverage_fixture")
				Eventually(session).Should(gexec.Exit(0))

				Ω(session.Out).Should(gbytes.Say("coverage: 71.4% of statements in github.com/onsi/ginkgo/integration/_fixtures/coverage_fixture, github.com/onsi/ginkgo/integration/_fixtures/coverage_fixture/external_coverage_fixture"))

				serialCoverProfileOutput, err = exec.Command("go", "tool", "cover", fmt.Sprintf("-func=%s", coverFile)).CombinedOutput()
				Ω(err).ShouldNot(HaveOccurred())

				removeSuccessfully("./_fixtures/coverage_fixture/coverage_fixture.coverprofile")

				Eventually(startGinkgo("./_fixtures/coverage_fixture", "-coverpkg=github.com/onsi/ginkgo/integration/_fixtures/coverage_fixture,github.com/onsi/ginkgo/integration/_fixtures/coverage_fixture/external_coverage_fixture", "-nodes=4")).Should(gexec.Exit(0))

				parallelCoverProfileOutput, err = exec.Command("go", "tool", "cover", fmt.Sprintf("-func=%s", coverFile)).CombinedOutput()
				Ω(err).ShouldNot(HaveOccurred())

				Ω(parallelCoverProfileOutput).Should(Equal(serialCoverProfileOutput))
			})
		})
	})

	Context("when a custom profile name is specified", func() {
		AfterEach(func() {
			removeSuccessfully("./_fixtures/coverage_fixture/coverage.txt")
		})

		It("generates cover profiles with the specified name", func() {
			session := startGinkgo("./_fixtures/coverage_fixture", "-cover", "-coverprofile=coverage.txt")
			Eventually(session).Should(gexec.Exit(0))

			Ω("./_fixtures/coverage_fixture/coverage.txt").Should(BeARegularFile())
		})
	})

	Context("when run in recursive mode", func() {
		AfterEach(func() {
			removeSuccessfully("./_fixtures/combined_coverage_fixture/coverage-recursive.txt")
			removeSuccessfully("./_fixtures/combined_coverage_fixture/first_package/coverage-recursive.txt")
			removeSuccessfully("./_fixtures/combined_coverage_fixture/second_package/coverage-recursive.txt")
		})

		It("generates a coverage file per package", func() {
			session := startGinkgo("./_fixtures/combined_coverage_fixture", "-r", "-cover", "-coverprofile=coverage-recursive.txt")
			Eventually(session).Should(gexec.Exit(0))

			Ω("./_fixtures/combined_coverage_fixture/first_package/coverage-recursive.txt").Should(BeARegularFile())
			Ω("./_fixtures/combined_coverage_fixture/second_package/coverage-recursive.txt").Should(BeARegularFile())
		})
	})

	Context("when run in parallel mode", func() {
		AfterEach(func() {
			removeSuccessfully("./_fixtures/coverage_fixture/coverage-parallel.txt")
		})

		It("works", func() {
			session := startGinkgo("./_fixtures/coverage_fixture", "-p", "-cover", "-coverprofile=coverage-parallel.txt")

			Eventually(session).Should(gexec.Exit(0))

			Ω("./_fixtures/coverage_fixture/coverage-parallel.txt").Should(BeARegularFile())
		})
	})

	Context("when run in recursive mode specifying a coverprofile", func() {
		AfterEach(func() {
			removeSuccessfully("./_fixtures/combined_coverage_fixture/coverprofile-recursive.txt")
			removeSuccessfully("./_fixtures/combined_coverage_fixture/first_package/coverprofile-recursive.txt")
			removeSuccessfully("./_fixtures/combined_coverage_fixture/second_package/coverprofile-recursive.txt")
		})

		It("combines the coverages", func() {
			session := startGinkgo("./_fixtures/combined_coverage_fixture", "-outputdir=./", "-r", "-cover", "-coverprofile=coverprofile-recursive.txt")
			Eventually(session).Should(gexec.Exit(0))

			By("generating a combined coverage file", func() {
				Ω("./_fixtures/combined_coverage_fixture/coverprofile-recursive.txt").Should(BeARegularFile())
			})

			By("also generating the single package coverage files", func() {
				Ω("./_fixtures/combined_coverage_fixture/first_package/coverprofile-recursive.txt").Should(BeARegularFile())
				Ω("./_fixtures/combined_coverage_fixture/second_package/coverprofile-recursive.txt").Should(BeARegularFile())
			})
		})
	})

	It("Fails with an error if output dir and coverprofile were set, but the output dir did not exist", func() {
		session := startGinkgo("./_fixtures/combined_coverage_fixture", "-outputdir=./all/profiles/here", "-r", "-cover", "-coverprofile=coverage.txt")

		Eventually(session).Should(gexec.Exit(1))
		output := session.Out.Contents()
		Ω(string(output)).Should(ContainSubstring("Unable to create combined profile, outputdir does not exist: ./all/profiles/here"))
	})

	Context("when only output dir was set", func() {
		AfterEach(func() {
			removeSuccessfully("./_fixtures/combined_coverage_fixture/first_package.coverprofile")
			removeSuccessfully("./_fixtures/combined_coverage_fixture/first_package/coverage.txt")
			removeSuccessfully("./_fixtures/combined_coverage_fixture/second_package.coverprofile")
			removeSuccessfully("./_fixtures/combined_coverage_fixture/second_package/coverage.txt")
		})
		It("moves coverages", func() {
			session := startGinkgo("./_fixtures/combined_coverage_fixture", "-outputdir=./", "-r", "-cover")
			Eventually(session).Should(gexec.Exit(0))

			Ω("./_fixtures/combined_coverage_fixture/first_package.coverprofile").Should(BeARegularFile())
			Ω("./_fixtures/combined_coverage_fixture/second_package.coverprofile").Should(BeARegularFile())
		})
	})
})
