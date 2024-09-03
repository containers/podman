package util

import (
	"testing"

	"github.com/containers/common/pkg/filters"
)

func TestMatchLabelFilters(t *testing.T) {
	testLabels := map[string]string{
		"label1": "",
		"label2": "test",
		"label3": "",
	}
	type args struct {
		filterValues []string
		labels       map[string]string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Match when all filters the same as labels",
			args: args{
				filterValues: []string{"label1", "label3", "label2=test"},
				labels:       testLabels,
			},
			want: true,
		},
		{
			name: "Match when filter value not provided in args",
			args: args{
				filterValues: []string{"label2"},
				labels:       testLabels,
			},
			want: true,
		},
		{
			name: "Match when no filter value is given",
			args: args{
				filterValues: []string{"label2="},
				labels:       testLabels,
			},
			want: true,
		},
		{
			name: "Do not match when filter value differs",
			args: args{
				filterValues: []string{"label2=differs"},
				labels:       testLabels,
			},
			want: false,
		},
		{
			name: "Do not match when filter value not listed in labels",
			args: args{
				filterValues: []string{"label1=xyz"},
				labels:       testLabels,
			},
			want: false,
		},
		{
			name: "Do not match when one from many not ok",
			args: args{
				filterValues: []string{"label1=xyz", "invalid=valid"},
				labels:       testLabels,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filters.MatchLabelFilters(tt.args.filterValues, tt.args.labels); got != tt.want {
				t.Errorf("MatchLabelFilters() = %v, want %v", got, tt.want)
			}
		})
	}
}
