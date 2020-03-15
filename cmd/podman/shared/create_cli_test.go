package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateSysctl(t *testing.T) {
	strSlice := []string{"net.core.test1=4", "kernel.msgmax=2"}
	result, _ := validateSysctl(strSlice)
	assert.Equal(t, result["net.core.test1"], "4")
}

func TestValidateSysctlBadSysctl(t *testing.T) {
	strSlice := []string{"BLAU=BLUE", "GELB^YELLOW"}
	_, err := validateSysctl(strSlice)
	assert.Error(t, err)
}
