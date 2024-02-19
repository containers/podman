package compression

import (
	"os"
	"testing"

	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/stretchr/testify/assert"
)

func Test_compressionFromFile(t *testing.T) {
	type args struct {
		path string
	}
	var tests = []struct {
		name string
		args args
		want ImageCompression
	}{
		{
			name: "xz",
			args: args{
				path: "/tmp/foo.xz",
			},
			want: Xz,
		},
		{
			name: "gzip",
			args: args{
				path: "/tmp/foo.gz",
			},
			want: Gz,
		},
		{
			name: "bz2",
			args: args{
				path: "/tmp/foo.bz2",
			},
			want: Bz2,
		},
		{
			name: "default is zstd",
			args: args{
				path: "/tmp/foo",
			},
			want: Zstd,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := KindFromFile(tt.args.path); got != tt.want {
				t.Errorf("KindFromFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestImageCompression_String(t *testing.T) {
	tests := []struct {
		name string
		c    ImageCompression
		want string
	}{
		{
			name: "xz",
			c:    Xz,
			want: "xz",
		},
		{
			name: "gz",
			c:    Gz,
			want: "gz",
		},
		{
			name: "bz2",
			c:    Bz2,
			want: "bz2",
		},
		{
			name: "zip",
			c:    Zip,
			want: "zip",
		},
		{
			name: "zstd is default",
			c:    99,
			want: "zst",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_Decompress(t *testing.T) {
	type args struct {
		src string
		dst string
	}

	type want string

	tests := []struct {
		name string
		args args
		want want
	}{
		{name: "zip", args: args{src: "./testfiles/sample.zip", dst: "./testfiles/hellozip"}, want: "zip\n"},
		{name: "xz", args: args{src: "./testfiles/sample.xz", dst: "./testfiles/helloxz"}, want: "xz\n"},
		{name: "gzip", args: args{src: "./testfiles/sample.gz", dst: "./testfiles/hellogz"}, want: "gzip\n"},
		{name: "bzip2", args: args{src: "./testfiles/sample.bz2", dst: "./testfiles/hellobz2"}, want: "bzip2\n"},
		{name: "zstd", args: args{src: "./testfiles/sample.zst", dst: "./testfiles/hellozstd"}, want: "zstd\n"},
		{name: "uncompressed", args: args{src: "./testfiles/sample.uncompressed", dst: "./testfiles/hellozuncompressed"}, want: "uncompressed\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srcVMFile := &define.VMFile{Path: tt.args.src}
			dstFilePath := tt.args.dst
			defer os.Remove(dstFilePath)
			err := Decompress(srcVMFile, dstFilePath)
			assert.NoError(t, err)
			data, err := os.ReadFile(dstFilePath)
			assert.NoError(t, err)
			assert.Equal(t, string(tt.want), string(data))
		})
	}
}
