package libpod

import (
	"testing"

	"github.com/containers/podman/v4/utils"
	"github.com/stretchr/testify/assert"
)

func TestRemoveScientificNotationFromFloat(t *testing.T) {
	numbers := []float64{0.0, .5, 1.99999932, 1.04e+10}
	results := []float64{0.0, .5, 1.99999932, 1.04}
	for i, x := range numbers {
		result, err := utils.RemoveScientificNotationFromFloat(x)
		assert.NoError(t, err)
		assert.Equal(t, result, results[i])
	}
}
