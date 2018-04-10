package formats

import (
	"bytes"
	"strings"
	"testing"

	"github.com/projectatomic/libpod/pkg/inspect"
)

func TestSetJSONFormatEncoder(t *testing.T) {
	tt := []struct {
		name       string
		imageData  *inspect.ImageData
		expected   string
		isTerminal bool
	}{
		{
			name:       "HTML tags are not escaped",
			imageData:  &inspect.ImageData{Author: "dave <dave@corp.io>"},
			expected:   `"Author": "dave <dave@corp.io>"`,
			isTerminal: true,
		},
		{
			name:       "HTML tags are escaped",
			imageData:  &inspect.ImageData{Author: "dave <dave@corp.io>"},
			expected:   `"Author": "dave \u003cdave@corp.io\u003e"`,
			isTerminal: false,
		},
	}

	for _, tc := range tt {
		buf := bytes.NewBuffer(nil)
		enc := setJSONFormatEncoder(tc.isTerminal, buf)
		if err := enc.Encode(tc.imageData); err != nil {
			t.Errorf("test %#v failed encoding: %s", tc.name, err)
		}
		if !strings.Contains(buf.String(), tc.expected) {
			t.Errorf("test %#v expected output to contain %#v. Output:\n%v\n", tc.name, tc.expected, buf.String())
		}
	}
}
