package util

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

func TestValidateSysctls(t *testing.T) {
	strSlice := []string{"net.core.test1=4", "kernel.msgmax=2"}
	result, _ := ValidateSysctls(strSlice)
	assert.Equal(t, result["net.core.test1"], "4")
}

func TestValidateSysctlBadSysctl(t *testing.T) {
	strSlice := []string{"BLAU=BLUE", "GELB^YELLOW"}
	_, err := ValidateSysctls(strSlice)
	assert.Error(t, err)
}

func TestValidateSysctlBadSysctlWithExtraSpaces(t *testing.T) {
	expectedError := "'%s' is invalid, extra spaces found"

	// should fail fast on first sysctl
	strSlice1 := []string{
		"net.ipv4.ping_group_range = 0 0",
		"net.ipv4.ping_group_range=0 0 ",
	}
	_, err := ValidateSysctls(strSlice1)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), fmt.Sprintf(expectedError, strSlice1[0]))

	// should fail on second sysctl
	strSlice2 := []string{
		"net.ipv4.ping_group_range=0 0",
		"net.ipv4.ping_group_range=0 0 ",
	}
	_, err = ValidateSysctls(strSlice2)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), fmt.Sprintf(expectedError, strSlice2[1]))
}

func TestCoresToPeriodAndQuota(t *testing.T) {
	cores := 1.0
	expectedPeriod := DefaultCPUPeriod
	expectedQuota := int64(DefaultCPUPeriod)

	actualPeriod, actualQuota := CoresToPeriodAndQuota(cores)
	assert.Equal(t, actualPeriod, expectedPeriod, "Period does not match")
	assert.Equal(t, actualQuota, expectedQuota, "Quota does not match")
}

func TestPeriodAndQuotaToCores(t *testing.T) {
	var (
		period        uint64 = 100000
		quota         int64  = 50000
		expectedCores        = 0.5
	)

	assert.Equal(t, PeriodAndQuotaToCores(period, quota), expectedCores)
}

func TestParseInputTime(t *testing.T) {
	tm, err := ParseInputTime("1.5", true)
	if err != nil {
		t.Errorf("expected error to be nil but was: %v", err)
	}

	expected, err := time.ParseInLocation(time.RFC3339Nano, "1970-01-01T00:00:01.500000000Z", time.UTC)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, expected, tm)
}
