//go:build amd64 || arm64

package machine

import (
	"testing"
)

func TestFCOSStream_String(t *testing.T) {
	tests := []struct {
		name string
		st   FCOSStream
		want string
	}{
		{
			name: "testing",
			st:   Testing,
			want: "testing",
		},
		{
			name: "stable",
			st:   Stable,
			want: "stable",
		},
		{
			name: "next",
			st:   Next,
			want: "next",
		},
		{
			name: "default is custom",
			st:   CustomStream,
			want: "custom",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.st.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}
