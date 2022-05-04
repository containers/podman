package utils

import (
	"net/url"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToLibpodFilters(t *testing.T) {
	good := url.Values{}
	good.Set("apple", "red")
	good.Set("banana", "yellow")
	good.Set("pear", "")
	goodResult := []string{"apple=red", "banana=yellow", "pear="}
	sort.Strings(goodResult)

	empty := url.Values{}
	type args struct {
		f url.Values
	}
	tests := []struct {
		name        string
		args        args
		wantFilters []string
	}{
		{
			name: "GoodURLValue",
			args: args{
				f: good,
			},
			wantFilters: goodResult,
		},
		{
			name: "Empty",
			args: args{
				f: empty,
			},
			wantFilters: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatchf(t, ToLibpodFilters(tt.args.f), tt.wantFilters, "ToLibpodFilters() = %v, want %v", ToLibpodFilters(tt.args.f), tt.wantFilters)
		})
	}
}

func TestToURLValues(t *testing.T) {
	good := url.Values{}
	good.Set("apple", "red")
	good.Set("banana", "yellow")
	good.Set("pear", "")
	goodResult := []string{"apple=red", "banana=yellow", "pear="}

	type args struct {
		f []string
	}
	tests := []struct {
		name        string
		args        args
		wantFilters url.Values
	}{
		{
			name:        "Good",
			args:        args{goodResult},
			wantFilters: good,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.EqualValuesf(t, ToURLValues(tt.args.f), tt.wantFilters, "ToURLValues() = %v, want %v", ToURLValues(tt.args.f), tt.wantFilters)
		})
	}
}
