package libpod

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

func TestRemoveScientificNotationFromFloat(t *testing.T) {
	numbers := []float64{0.0, .5, 1.99999932, 1.04e+10}
	results := []float64{0.0, .5, 1.99999932, 1.04}
	for i, x := range numbers {
		result, err := RemoveScientificNotationFromFloat(x)
		assert.NoError(t, err)
		assert.Equal(t, result, results[i])
	}
}
