package shim

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_validateDestinationPaths(t *testing.T) {
	type args struct {
		dest string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "Expect fail - /tmp",
			args:    args{"/tmp"},
			wantErr: true,
		},
		{
			name: "Expect fail trailing /",
			args: args{
				dest: "/tmp/",
			},
			wantErr: true,
		},
		{
			name: "Expect fail double /",
			args: args{
				dest: "//tmp",
			},
			wantErr: true,
		},
		{
			name: "/var should fail",
			args: args{
				dest: "/var",
			},
			wantErr: true,
		},
		{
			name: "/etc should fail",
			args: args{
				dest: "/etc",
			},
			wantErr: true,
		},
		{
			name: "/tmp subdir OK",
			args: args{
				dest: "/tmp/foobar",
			},
			wantErr: false,
		},
		{
			name: "/foobar OK",
			args: args{
				dest: "/foobar",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDestinationPaths(tt.args.dest)
			if tt.wantErr {
				assert.Error(t, err, "illegal mount target should trip error")
				assert.ErrorContainsf(t, err, "onsider another location or a subdirectory of an existing location", "illegal mount target")
			}
		})
	}
}
