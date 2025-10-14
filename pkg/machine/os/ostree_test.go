package os

import (
	"testing"

	"github.com/blang/semver/v4"
)

func Test_compareMajorMinor(t *testing.T) {
	type args struct {
		versionA semver.Version
		versionB semver.Version
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "equal major and minor versions and different patch",
			args: args{
				versionA: semver.MustParse("1.2.3"),
				versionB: semver.MustParse("1.2.5"),
			},
			want: 0,
		},
		{
			name: "A major version less than B",
			args: args{
				versionA: semver.MustParse("1.5.0"),
				versionB: semver.MustParse("2.5.0"),
			},
			want: -1,
		},
		{
			name: "A major version greater than B",
			args: args{
				versionA: semver.MustParse("3.2.0"),
				versionB: semver.MustParse("2.9.0"),
			},
			want: 1,
		},
		{
			name: "A minor version less than B (same major)",
			args: args{
				versionA: semver.MustParse("1.2.0"),
				versionB: semver.MustParse("1.5.0"),
			},
			want: -1,
		},
		{
			name: "A minor version greater than B (same major)",
			args: args{
				versionA: semver.MustParse("1.8.0"),
				versionB: semver.MustParse("1.3.0"),
			},
			want: 1,
		},
		{
			name: "completely equal versions",
			args: args{
				versionA: semver.MustParse("1.2.3"),
				versionB: semver.MustParse("1.2.3"),
			},
			want: 0,
		},
		{
			name: "zero versions",
			args: args{
				versionA: semver.MustParse("0.0.0"),
				versionB: semver.MustParse("0.0.1"),
			},
			want: 0,
		},
		{
			name: "A is zero, B is not",
			args: args{
				versionA: semver.MustParse("0.0.0"),
				versionB: semver.MustParse("1.0.0"),
			},
			want: -1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compareMajorMinor(tt.args.versionA, tt.args.versionB); got != tt.want {
				t.Errorf("compareMajorMinor() = %v, want %v", got, tt.want)
			}
		})
	}
}
