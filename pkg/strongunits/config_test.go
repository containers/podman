package strongunits

import "testing"

func TestGiB_toBytes(t *testing.T) {
	tests := []struct {
		name string
		g    GiB
		want B
	}{
		{
			name: "good-1",
			g:    1,
			want: 1073741824,
		},
		{
			name: "good-2",
			g:    2,
			want: 2147483648,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.g.ToBytes(); got != tt.want {
				t.Errorf("ToBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKiB_toBytes(t *testing.T) {
	tests := []struct {
		name string
		k    KiB
		want B
	}{
		{
			name: "good-1",
			k:    100,
			want: 102400,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.k.ToBytes(); got != tt.want {
				t.Errorf("ToBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMiB_toBytes(t *testing.T) {
	tests := []struct {
		name string
		m    MiB
		want B
	}{
		{
			name: "good-1",
			m:    1024,
			want: 1073741824,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.m.ToBytes(); got != tt.want {
				t.Errorf("ToBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToGiB(t *testing.T) {
	type args struct {
		b StorageUnits
	}
	tests := []struct {
		name string
		args args
		want GiB
	}{
		{
			name: "bytes to gib",
			args: args{B(5368709120)},
			want: 5,
		},
		{
			name: "kib to gib",
			args: args{KiB(3145728 * 2)},
			want: 6,
		},
		{
			name: "mib to gib",
			args: args{MiB(2048)},
			want: 2,
		},
		{
			name: "gib to gib",
			args: args{GiB(2)},
			want: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToGiB(tt.args.b); got != tt.want {
				t.Errorf("ToGiB() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToKiB(t *testing.T) {
	type args struct {
		b StorageUnits
	}
	tests := []struct {
		name string
		args args
		want KiB
	}{
		{
			name: "bytes to kib",
			args: args{B(1024)},
			want: 1,
		},
		{
			name: "mib to kib",
			args: args{MiB(2)},
			want: 2048,
		},
		{
			name: "kib to kib",
			args: args{KiB(800)},
			want: 800,
		},
		{
			name: "gib to mib",
			args: args{GiB(3)},
			want: 3145728,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToKiB(tt.args.b); got != tt.want {
				t.Errorf("ToKiB() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToMib(t *testing.T) {
	type args struct {
		b StorageUnits
	}
	tests := []struct {
		name string
		args args
		want MiB
	}{
		{
			name: "bytes to mib",
			args: args{B(3145728)},
			want: 3,
		},
		{
			name: "kib to mib",
			args: args{KiB(2048)},
			want: 2,
		},
		{
			name: "mib to mib",
			args: args{MiB(2)},
			want: 2,
		},
		{
			name: "gib to mib",
			args: args{GiB(3)},
			want: 3072,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToMib(tt.args.b); got != tt.want {
				t.Errorf("ToMib() = %v, want %v", got, tt.want)
			}
		})
	}
}
