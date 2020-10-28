package parse

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnforceRange(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"{{range .}}{{.ID}}{{end}}", "{{range .}}{{.ID}}{{end}}"},
		{"{{.ID}}", "{{range .}}{{.ID}}{{end}}"},
		{"{{ range . }}{{ .ID }}{{ end }}", "{{ range . }}{{ .ID }}{{ end }}"},
		// EnforceRange does not verify syntax or semantics, that will happen later
		{"{{range .}}{{.ID}}", "{{range .}}{{range .}}{{.ID}}{{end}}"},
		{".ID", "{{range .}}.ID{{end}}"},
	}

	for _, tc := range tests {
		tc := tc
		label := "TestEnforceRange_" + tc.input
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, EnforceRange(tc.input))
		})
	}
}
