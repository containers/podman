package util

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

var (
	sliceData = []string{"one", "two", "three", "four"}
)

func TestStringInSlice(t *testing.T) {
	// string is in the slice
	assert.True(t, StringInSlice("one", sliceData))
	// string is not in the slice
	assert.False(t, StringInSlice("five", sliceData))
	// string is not in empty slice
	assert.False(t, StringInSlice("one", []string{}))
}
