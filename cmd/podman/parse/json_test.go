package parse

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchesJSONFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"json", true},
		{" json", true},
		{" json ", true},
		{"  json   ", true},
		{"{{json}}", true},
		{"{{json }}", true},
		{"{{json .}}", true},
		{"{{ json .}}", true},
		{"{{ json . }}", true},
		{"  {{   json   .  }}  ", true},
		{"{{ json .", false},
		{"json . }}", false},
		{"{{.ID }} json .", false},
		{"json .", false},
		{"{{json.}}", true},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, MatchesJSONFormat(tt.input))
	}

	for _, tc := range tests {
		tc := tc
		label := "MatchesJSONFormat/" + strings.ReplaceAll(tc.input, " ", "_")
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, MatchesJSONFormat(tc.input), fmt.Sprintf("Scanning %q failed", tc.input))
		})
	}
}
