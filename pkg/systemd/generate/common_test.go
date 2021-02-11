package generate

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterPodFlags(t *testing.T) {
	tests := []struct {
		input []string
	}{
		{[]string{"podman", "pod", "create"}},
		{[]string{"podman", "pod", "create", "--name", "foo"}},
		{[]string{"podman", "pod", "create", "--pod-id-file", "foo"}},
		{[]string{"podman", "pod", "create", "--pod-id-file=foo"}},
		{[]string{"podman", "run", "--pod", "foo"}},
		{[]string{"podman", "run", "--pod=foo"}},
	}

	for _, test := range tests {
		processed := filterPodFlags(test.input)
		for _, s := range processed {
			assert.False(t, strings.HasPrefix(s, "--pod-id-file"))
			assert.False(t, strings.HasPrefix(s, "--pod"))
		}
	}
}

func TestEscapeSystemdArguments(t *testing.T) {
	tests := []struct {
		input  []string
		output []string
	}{
		{
			[]string{"foo", "bar=\"arg\""},
			[]string{"foo", "bar=\"arg\""},
		},
		{
			[]string{"foo", "bar=\"arg with space\""},
			[]string{"foo", "\"bar=\\\"arg with space\\\"\""},
		},
		{
			[]string{"foo", "bar=\"arg with\ttab\""},
			[]string{"foo", "\"bar=\\\"arg with\\ttab\\\"\""},
		},
		{
			[]string{"$"},
			[]string{"$$"},
		},
		{
			[]string{"foo", "command with dollar sign $"},
			[]string{"foo", "\"command with dollar sign $$\""},
		},
		{
			[]string{"foo", "command with two dollar signs $$"},
			[]string{"foo", "\"command with two dollar signs $$$$\""},
		},
		{
			[]string{"%"},
			[]string{"%%"},
		},
		{
			[]string{"foo", "command with percent sign %"},
			[]string{"foo", "\"command with percent sign %%\""},
		},
		{
			[]string{"foo", "command with two percent signs %%"},
			[]string{"foo", "\"command with two percent signs %%%%\""},
		},
		{
			[]string{`\`},
			[]string{`\\`},
		},
		{
			[]string{"foo", `command with backslash \`},
			[]string{"foo", `"command with backslash \\"`},
		},
		{
			[]string{"foo", `command with two backslashs \\`},
			[]string{"foo", `"command with two backslashs \\\\"`},
		},
	}

	for _, test := range tests {
		quoted := escapeSystemdArguments(test.input)
		assert.Equal(t, test.output, quoted)
	}
}
