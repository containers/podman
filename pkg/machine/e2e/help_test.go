package e2e_test

import (
	"regexp"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman help", func() {
	It("podman usage base command is podman or podman-remote, without extension	", func() {
		helpSession, err := mb.setCmd(new(helpMachine)).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(helpSession).Should(Exit(0))

		// Verify `.exe` suffix doesn't present in the usage command string
		helpMessages := helpSession.outputToStringSlice()
		usageCmdIndex := slices.IndexFunc(helpMessages, func(helpMessage string) bool { return helpMessage == "Usage:" }) + 1
		Expect(regexp.MustCompile(`\w\.exe\b`).MatchString(helpMessages[usageCmdIndex])).Should(BeFalse())
	})
})
