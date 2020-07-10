package parse

import (
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
		{"json ", true},
		{"  json   ", true},
		{"{{json .}}", true},
		{"{{ json .}}", true},
		{"{{json .   }}", true},
		{"  {{  json .    }}   ", true},
		{"{{json }}", false},
		{"{{json .", false},
		{"json . }}", false},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, MatchesJSONFormat(tt.input))
	}
}
