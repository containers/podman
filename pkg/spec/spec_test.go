package createconfig

import (
	"runtime"
	"testing"

	"github.com/containers/libpod/pkg/cgroups"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/sysinfo"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/idtools"
	"github.com/docker/go-units"
	"github.com/stretchr/testify/assert"
)

var (
	sysInfo = sysinfo.New(true)
)

// Make createconfig to test with
func makeTestCreateConfig() *CreateConfig {
	cc := new(CreateConfig)
	cc.Resources = CreateResourceConfig{}
	cc.User.IDMappings = new(storage.IDMappingOptions)
	cc.User.IDMappings.UIDMap = []idtools.IDMap{}
	cc.User.IDMappings.GIDMap = []idtools.IDMap{}

	return cc
}

func doCommonSkipChecks(t *testing.T) {
	// The default configuration of podman enables seccomp, which is not available on non-Linux systems.
	// Thus, any tests that use the default seccomp setting would fail.
	// Skip the tests on non-Linux platforms rather than explicitly disable seccomp in the test and possibly affect the test result.
	if runtime.GOOS != "linux" {
		t.Skip("seccomp, which is enabled by default, is only supported on Linux")
	}

	if rootless.IsRootless() {
		isCgroupV2, err := cgroups.IsCgroup2UnifiedMode()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !isCgroupV2 {
			t.Skip("cgroups v1 cannot be used when rootless")
		}
	}
}

// TestPIDsLimit verifies the given pid-limit is correctly defined in the spec
func TestPIDsLimit(t *testing.T) {
	doCommonSkipChecks(t)

	if !sysInfo.PidsLimit {
		t.Skip("running test not supported by the host system")
	}

	cc := makeTestCreateConfig()
	cc.Resources.PidsLimit = 22

	spec, err := cc.createConfigToOCISpec(nil, nil)
	assert.NoError(t, err)

	assert.Equal(t, spec.Linux.Resources.Pids.Limit, int64(22))
}

// TestBLKIOWeightDevice verifies the given blkio weight is correctly set in the
// spec.
func TestBLKIOWeightDevice(t *testing.T) {
	doCommonSkipChecks(t)

	if !sysInfo.BlkioWeightDevice {
		t.Skip("running test not supported by the host system")
	}

	cc := makeTestCreateConfig()
	cc.Resources.BlkioWeightDevice = []string{"/dev/zero:100"}

	spec, err := cc.createConfigToOCISpec(nil, nil)
	assert.NoError(t, err)

	// /dev/zero is guaranteed 1,5 by the Linux kernel
	assert.Equal(t, spec.Linux.Resources.BlockIO.WeightDevice[0].Major, int64(1))
	assert.Equal(t, spec.Linux.Resources.BlockIO.WeightDevice[0].Minor, int64(5))
	assert.Equal(t, *(spec.Linux.Resources.BlockIO.WeightDevice[0].Weight), uint16(100))
}

// TestMemorySwap verifies that the given swap memory limit is correctly set in
// the spec.
func TestMemorySwap(t *testing.T) {
	doCommonSkipChecks(t)

	if !sysInfo.SwapLimit {
		t.Skip("running test not supported by the host system")
	}

	swapLimit, err := units.RAMInBytes("45m")
	assert.NoError(t, err)

	cc := makeTestCreateConfig()
	cc.Resources.MemorySwap = swapLimit

	spec, err := cc.createConfigToOCISpec(nil, nil)
	assert.NoError(t, err)

	assert.Equal(t, *(spec.Linux.Resources.Memory.Swap), swapLimit)
}
