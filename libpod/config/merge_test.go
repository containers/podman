package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeStrings(t *testing.T) {
	testData := []struct {
		a   string
		b   string
		res string
	}{
		{"", "", ""},
		{"a", "", "a"},
		{"a", "b", "a"},
		{"", "b", "b"},
	}
	for _, data := range testData {
		res := mergeStrings(data.a, data.b)
		assert.Equal(t, data.res, res)
	}
}

func TestMergeStringSlices(t *testing.T) {
	testData := []struct {
		a   []string
		b   []string
		res []string
	}{
		{
			nil, nil, nil,
		},
		{
			nil,
			[]string{},
			[]string{},
		},
		{
			[]string{},
			nil,
			[]string{},
		},
		{
			[]string{},
			[]string{},
			[]string{},
		},
		{
			[]string{"a"},
			[]string{},
			[]string{"a"},
		},
		{
			[]string{"a"},
			[]string{"b"},
			[]string{"a"},
		},
		{
			[]string{},
			[]string{"b"},
			[]string{"b"},
		},
	}
	for _, data := range testData {
		res := mergeStringSlices(data.a, data.b)
		assert.Equal(t, data.res, res)
	}
}

func TestMergeStringMaps(t *testing.T) {
	testData := []struct {
		a   map[string][]string
		b   map[string][]string
		res map[string][]string
	}{
		{
			nil, nil, nil,
		},
		{
			nil,
			map[string][]string{},
			map[string][]string{}},
		{
			map[string][]string{"a": {"a"}},
			nil,
			map[string][]string{"a": {"a"}},
		},
		{
			nil,
			map[string][]string{"b": {"b"}},
			map[string][]string{"b": {"b"}},
		},
		{
			map[string][]string{"a": {"a"}},
			map[string][]string{"b": {"b"}},
			map[string][]string{"a": {"a"}},
		},
	}
	for _, data := range testData {
		res := mergeStringMaps(data.a, data.b)
		assert.Equal(t, data.res, res)
	}
}

func TestMergeInts64(t *testing.T) {
	testData := []struct {
		a   int64
		b   int64
		res int64
	}{
		{int64(0), int64(0), int64(0)},
		{int64(1), int64(0), int64(1)},
		{int64(0), int64(1), int64(1)},
		{int64(2), int64(1), int64(2)},
		{int64(-1), int64(1), int64(-1)},
		{int64(0), int64(-1), int64(-1)},
	}
	for _, data := range testData {
		res := mergeInt64s(data.a, data.b)
		assert.Equal(t, data.res, res)
	}
}
func TestMergeUint32(t *testing.T) {
	testData := []struct {
		a   uint32
		b   uint32
		res uint32
	}{
		{uint32(0), uint32(0), uint32(0)},
		{uint32(1), uint32(0), uint32(1)},
		{uint32(0), uint32(1), uint32(1)},
		{uint32(2), uint32(1), uint32(2)},
	}
	for _, data := range testData {
		res := mergeUint32s(data.a, data.b)
		assert.Equal(t, data.res, res)
	}
}

func TestMergeBools(t *testing.T) {
	testData := []struct {
		a   bool
		b   bool
		res bool
	}{
		{false, false, false},
		{true, false, true},
		{false, true, true},
		{true, true, true},
	}
	for _, data := range testData {
		res := mergeBools(data.a, data.b)
		assert.Equal(t, data.res, res)
	}
}
