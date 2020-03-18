package autoupdate

import (
	"testing"
)

func TestValidateImageReference(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{ // Fully-qualified reference
			input: "quay.io/foo/bar:tag",
			valid: true,
		},
		{ // Fully-qualified reference in transport notation
			input: "docker://quay.io/foo/bar:tag",
			valid: true,
		},
		{ // Fully-qualified reference but with digest
			input: "quay.io/foo/bar@sha256:c9b1b535fdd91a9855fb7f82348177e5f019329a58c53c47272962dd60f71fc9",
			valid: false,
		},
		{ // Reference with missing tag
			input: "quay.io/foo/bar",
			valid: false,
		},
		{ // Short name
			input: "alpine",
			valid: false,
		},
		{ // Short name with repo
			input: "library/alpine",
			valid: false,
		},
		{ // Wrong transport
			input: "docker-archive:/some/path.tar",
			valid: false,
		},
	}

	for _, test := range tests {
		err := ValidateImageReference(test.input)
		if test.valid && err != nil {
			t.Fatalf("parsing %q should have succeeded: %v", test.input, err)
		} else if !test.valid && err == nil {
			t.Fatalf("parsing %q should have failed", test.input)
		}
	}
}
