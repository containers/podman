package images

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildMatchIID(t *testing.T) {
	assert.True(t, iidRegex.MatchString("a883dafc480d466ee04e0d6da986bd78eb1fdd2178d04693723da3a8f95d42f4"))
	assert.True(t, iidRegex.MatchString("3da3a8f95d42"))
	assert.False(t, iidRegex.MatchString("3da3"))
}

func TestBuildNotMatchStatusMessage(t *testing.T) {
	assert.False(t, iidRegex.MatchString("Copying config a883dafc480d466ee04e0d6da986bd78eb1fdd2178d04693723da3a8f95d42f4"))
}
