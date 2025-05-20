//go:build !remote

package libpod

import (
	"testing"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
)

func Test_sortMounts(t *testing.T) {
	tests := []struct {
		name string
		args []spec.Mount
		want []spec.Mount
	}{
		{
			name: "simple nested mounts",
			args: []spec.Mount{
				{
					Destination: "/abc/123",
				},
				{
					Destination: "/abc",
				},
			},
			want: []spec.Mount{
				{
					Destination: "/abc",
				},
				{
					Destination: "/abc/123",
				},
			},
		},
		{
			name: "root mount",
			args: []spec.Mount{
				{
					Destination: "/abc",
				},
				{
					Destination: "/",
				},
				{
					Destination: "/def",
				},
			},
			want: []spec.Mount{
				{
					Destination: "/",
				},
				{
					Destination: "/abc",
				},
				{
					Destination: "/def",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortMounts(tt.args)
			assert.Equal(t, tt.want, got)
		})
	}
}
