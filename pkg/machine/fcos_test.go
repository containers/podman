package machine

import "testing"

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
			name: "default is xz",
			args: args{
				path: "/tmp/foo",
			},
			want: Xz,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compressionFromFile(tt.args.path); got != tt.want {
				t.Errorf("compressionFromFile() = %v, want %v", got, tt.want)
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
			name: "xz is default",
			c:    99,
			want: "xz",
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

func TestImageFormat_String(t *testing.T) {
	tests := []struct {
		name string
		imf  ImageFormat
		want string
	}{
		{
			name: "vhdx.zip",
			imf:  Vhdx,
			want: "vhdx.zip",
		},
		{
			name: "qcow2",
			imf:  Qcow,
			want: "qcow2.xz",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.imf.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_artifact_String(t *testing.T) {
	tests := []struct {
		name string
		a    Artifact
		want string
	}{
		{
			name: "qemu",
			a:    Qemu,
			want: "qemu",
		},
		{
			name: "hyperv",
			a:    HyperV,
			want: "hyperv",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
			name: "default is stable",
			st:   99,
			want: "stable",
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
