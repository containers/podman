package integration

import (
	"fmt"

	. "github.com/containers/podman/v5/test/utils"
	"github.com/containers/podman/v5/version"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman version", func() {

	It("podman version", func() {
		session := podmanTest.Podman([]string{"version"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.Out.Contents()).Should(ContainSubstring(version.Version.String()))
	})

	It("podman -v", func() {
		session := podmanTest.Podman([]string{"-v"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.Out.Contents()).Should(ContainSubstring(version.Version.String()))
	})

	It("podman --version", func() {
		session := podmanTest.Podman([]string{"--version"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.Out.Contents()).Should(ContainSubstring(version.Version.String()))
	})

	It("podman version --format json", func() {
		tests := []struct {
			input    string
			success  bool
			exitCode int
		}{
			{"json", true, 0},
			{" json", true, 0},
			{"json ", true, 0},
			{"  json   ", true, 0},
			{"{{json .}}", true, 0},
			{"{{ json .}}", true, 0},
			{"{{json .   }}", true, 0},
			{"  {{  json .    }}   ", true, 0},
			{"{{json }}", true, 0},
			{"{{json .", false, 125},
			{"json . }}", false, 0}, // without opening {{ template seen as string literal
		}
		for _, tt := range tests {
			session := podmanTest.Podman([]string{"version", "--format", tt.input})
			session.WaitWithDefaultTimeout()

			desc := fmt.Sprintf("JSON test(%q)", tt.input)
			Expect(session).Should(Exit(tt.exitCode), desc)
			Expect(session.IsJSONOutputValid()).To(Equal(tt.success), desc)
		}
	})

	It("podman version --format GO template", func() {
		session := podmanTest.Podman([]string{"version", "--format", "{{ .Client.Version }}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"version", "--format", "{{ .Client.Os }}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"version", "--format", "{{ .Server.Version }}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"version", "--format", "{{ .Server.Os }}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"version", "--format", "{{ .Version }}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman help", func() {
		session := podmanTest.Podman([]string{"help"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.Out.Contents()).Should(
			ContainSubstring("Display the Podman version information"),
		)
	})
})
