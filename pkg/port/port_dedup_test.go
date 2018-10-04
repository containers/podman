package port

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddPortToMappingInvalidCtrPort(t *testing.T) {
	invalidPorts := []int32{-50, 65537}

	for _, port := range invalidPorts {
		testPort := PortMapping{
			ContainerPort: port,
			HostPort:      50,
			Length:        1,
			Protocol:      "tcp",
		}

		_, err := AddPortToMapping(testPort, []PortMapping{})
		assert.Error(t, err)
	}
}

func TestAddPortToMappingInvalidHostPort(t *testing.T) {
	invalidPorts := []int32{-50, 65537}

	for _, port := range invalidPorts {
		testPort := PortMapping{
			ContainerPort: 50,
			HostPort:      port,
			Length:        1,
			Protocol:      "tcp",
		}

		_, err := AddPortToMapping(testPort, []PortMapping{})
		assert.Error(t, err)
	}
}

func TestAddPortToMappingInvalidLength(t *testing.T) {
	testPort := PortMapping{
		ContainerPort: 50,
		HostPort:      50,
		Length:        0,
		Protocol:      "tcp",
	}

	_, err := AddPortToMapping(testPort, []PortMapping{})
	assert.Error(t, err)
}

func TestAddPortToMappingCtrRangeTooLong(t *testing.T) {
	testPort := PortMapping{
		ContainerPort: 65535,
		HostPort:      50,
		Length:        50,
		Protocol:      "tcp",
	}

	_, err := AddPortToMapping(testPort, []PortMapping{})
	assert.Error(t, err)

}

func TestAddPortToMappingHostRangeTooLong(t *testing.T) {
	testPort := PortMapping{
		ContainerPort: 50,
		HostPort:      65535,
		Length:        50,
		Protocol:      "tcp",
	}

	_, err := AddPortToMapping(testPort, []PortMapping{})
	assert.Error(t, err)
}

func TestAddPortToMappingBadProtocol(t *testing.T) {
	testPort := PortMapping{
		ContainerPort: 50,
		HostPort:      50,
		Length:        0,
		Protocol:      "asdf",
	}

	_, err := AddPortToMapping(testPort, []PortMapping{})
	assert.Error(t, err)
}

func TestAddPortToMappingSinglePort(t *testing.T) {
	protocols := []string{"tcp", "udp"}
	lengths := []uint16{1, 65535}

	for _, proto := range protocols {
		for _, length := range lengths {
			testPort := PortMapping{
				ContainerPort: 0,
				HostPort:      0,
				Length:        length,
				Protocol:      proto,
			}

			ports, err := AddPortToMapping(testPort, []PortMapping{})
			require.NoError(t, err)
			require.Equal(t, 1, len(ports))
			assert.EqualValues(t, testPort, ports[0])
		}
	}
}

func TestAddPortToMappingTwoPortsNoOverlap(t *testing.T) {
	testPort1 := PortMapping{
		ContainerPort: 0,
		HostPort:      0,
		Length:        1,
		Protocol:      "tcp",
	}

	testPort2 := PortMapping{
		ContainerPort: 5,
		HostPort:      5,
		Length:        1,
		Protocol:      "tcp",
	}

	ports1, err := AddPortToMapping(testPort1, []PortMapping{})
	require.NoError(t, err)
	require.Equal(t, 1, len(ports1))
	assert.EqualValues(t, testPort1, ports1[0])

	ports2, err := AddPortToMapping(testPort2, ports1)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(ports2))
}

func TestAddPortToMappingThreePortsNoOverlap(t *testing.T) {
	testPort1 := PortMapping{
		ContainerPort: 0,
		HostPort:      0,
		Length:        1,
		Protocol:      "tcp",
	}

	testPort2 := PortMapping{
		ContainerPort: 5,
		HostPort:      5,
		Length:        1,
		Protocol:      "tcp",
	}

	testPort3 := PortMapping{
		ContainerPort: 10,
		HostPort:      10,
		Length:        1,
		Protocol:      "tcp",
	}

	ports1, err := AddPortToMapping(testPort1, []PortMapping{})
	require.NoError(t, err)
	require.Equal(t, 1, len(ports1))
	assert.EqualValues(t, testPort1, ports1[0])

	ports2, err := AddPortToMapping(testPort2, ports1)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(ports2))

	ports3, err := AddPortToMapping(testPort3, ports2)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(ports3))
}

func TestAddPortToMappingConflictingCtrMappingErrors(t *testing.T) {
	testPort1 := PortMapping{
		ContainerPort: 0,
		HostPort:      0,
		Length:        1,
		Protocol:      "tcp",
	}

	testPort2 := PortMapping{
		ContainerPort: 0,
		HostPort:      5,
		Length:        1,
		Protocol:      "tcp",
	}

	ports, err := AddPortToMapping(testPort1, []PortMapping{})
	require.NoError(t, err)
	require.Equal(t, 1, len(ports))
	assert.EqualValues(t, testPort1, ports[0])

	_, err = AddPortToMapping(testPort2, ports)
	assert.Error(t, err)
}

func TestAddPortToMappingConflictingHostMappingErrors(t *testing.T) {
	testPort1 := PortMapping{
		ContainerPort: 0,
		HostPort:      0,
		Length:        1,
		Protocol:      "tcp",
	}

	testPort2 := PortMapping{
		ContainerPort: 5,
		HostPort:      0,
		Length:        1,
		Protocol:      "tcp",
	}

	ports, err := AddPortToMapping(testPort1, []PortMapping{})
	require.NoError(t, err)
	require.Equal(t, 1, len(ports))
	assert.EqualValues(t, testPort1, ports[0])

	_, err = AddPortToMapping(testPort2, ports)
	assert.Error(t, err)
}

func TestAddPortToMappingSameMappingDifferentProtocolSucceeds(t *testing.T) {
	testPort1 := PortMapping{
		ContainerPort: 0,
		HostPort:      0,
		Length:        1,
		Protocol:      "tcp",
	}

	testPort2 := PortMapping{
		ContainerPort: 0,
		HostPort:      0,
		Length:        1,
		Protocol:      "udp",
	}

	ports1, err := AddPortToMapping(testPort1, []PortMapping{})
	require.NoError(t, err)
	require.Equal(t, 1, len(ports1))
	assert.EqualValues(t, testPort1, ports1[0])

	ports2, err := AddPortToMapping(testPort2, ports1)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(ports2))
}

func TestAddPortToMappingAdjacentMappingsToDifferentRangesSucceeds(t *testing.T) {
	testPort1 := PortMapping{
		ContainerPort: 0,
		HostPort:      0,
		Length:        1,
		Protocol:      "tcp",
	}

	testPort2 := PortMapping{
		ContainerPort: 1,
		HostPort:      2,
		Length:        1,
		Protocol:      "tcp",
	}

	ports1, err := AddPortToMapping(testPort1, []PortMapping{})
	require.NoError(t, err)
	require.Equal(t, 1, len(ports1))
	assert.EqualValues(t, testPort1, ports1[0])

	ports2, err := AddPortToMapping(testPort2, ports1)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(ports2))
}

func TestAddPortToMappingAdjacentMappingsToSameRangeCombines(t *testing.T) {
	testPort1 := PortMapping{
		ContainerPort: 0,
		HostPort:      0,
		Length:        1,
		Protocol:      "tcp",
	}

	testPort2 := PortMapping{
		ContainerPort: 1,
		HostPort:      1,
		Length:        1,
		Protocol:      "tcp",
	}

	expectedPort := PortMapping{
		ContainerPort: 0,
		HostPort:      0,
		Length:        2,
		Protocol:      "tcp",
	}

	ports1, err := AddPortToMapping(testPort1, []PortMapping{})
	require.NoError(t, err)
	require.Equal(t, 1, len(ports1))
	assert.EqualValues(t, testPort1, ports1[0])

	ports2, err := AddPortToMapping(testPort2, ports1)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ports2))
	assert.EqualValues(t, expectedPort, ports2[0])
}

func TestAddPortToMappingThreeAdjacentMappingsCombine(t *testing.T) {
	testPort1 := PortMapping{
		ContainerPort: 0,
		HostPort:      0,
		Length:        1,
		Protocol:      "tcp",
	}

	testPort2 := PortMapping{
		ContainerPort: 1,
		HostPort:      1,
		Length:        1,
		Protocol:      "tcp",
	}

	testPort3 := PortMapping{
		ContainerPort: 2,
		HostPort:      2,
		Length:        1,
		Protocol:      "tcp",
	}

	expectedPort := PortMapping{
		ContainerPort: 0,
		HostPort:      0,
		Length:        3,
		Protocol:      "tcp",
	}

	ports1, err := AddPortToMapping(testPort1, []PortMapping{})
	require.NoError(t, err)
	require.Equal(t, 1, len(ports1))
	assert.EqualValues(t, testPort1, ports1[0])

	ports2, err := AddPortToMapping(testPort2, ports1)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ports2))

	ports3, err := AddPortToMapping(testPort3, ports2)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ports3))
	assert.EqualValues(t, expectedPort, ports3[0])
}
