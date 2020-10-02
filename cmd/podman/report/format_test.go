package report

import (
	"strings"
	"testing"
)

func TestNormalizeFormat(t *testing.T) {
	cases := []struct {
		format   string
		expected string
	}{
		{"table {{.ID}}", "{{.ID}}\n"},
		{"table {{.ID}} {{.C}}", "{{.ID}}\t{{.C}}\n"},
		{"{{.ID}}", "{{.ID}}\n"},
		{"{{.ID}}\n", "{{.ID}}\n"},
		{"{{.ID}} {{.C}}", "{{.ID}} {{.C}}\n"},
		{"\t{{.ID}}", "\t{{.ID}}\n"},
		{`\t` + "{{.ID}}", "\t{{.ID}}\n"},
		{"table {{.ID}}\t{{.C}}", "{{.ID}}\t{{.C}}\n"},
		{"{{.ID}} table {{.C}}", "{{.ID}} table {{.C}}\n"},
	}
	for _, tc := range cases {
		tc := tc

		label := strings.ReplaceAll(tc.format, " ", "<sp>")
		t.Run("NormalizeFormat/"+label, func(t *testing.T) {
			t.Parallel()
			actual := NormalizeFormat(tc.format)
			if actual != tc.expected {
				t.Errorf("Expected %q, actual %q", tc.expected, actual)
			}
		})
	}
}
