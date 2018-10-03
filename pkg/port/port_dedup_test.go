package port

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddPortToMappingInvalidCtrPort(t *testing.T) {
	invalidPorts := []int32{-50, 65537}

	for _, port := range invalidPorts {
		testPort := PortMapping{
			ContainerPort: port,
			HostPort: 50,
			Length: 1,
		}

		_, err := AddPortToMapping(testPort, []libpod.PortMapping{})
		assert.Error(t, err)
	}
}
