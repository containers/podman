//go:build !remote

package kube

import (
	"testing"

	v1 "github.com/containers/podman/v5/pkg/k8s.io/api/core/v1"
	"github.com/containers/podman/v5/pkg/k8s.io/apimachinery/pkg/util/intstr"
	"github.com/stretchr/testify/assert"
)

func testPropagation(t *testing.T, propagation v1.MountPropagationMode, expected string) {
	dest, options, err := parseMountPath("/to", false, &propagation)
	assert.NoError(t, err)
	assert.Equal(t, dest, "/to")
	assert.Contains(t, options, expected)
}

func TestParseMountPathPropagation(t *testing.T) {
	testPropagation(t, v1.MountPropagationNone, "private")
	testPropagation(t, v1.MountPropagationHostToContainer, "rslave")
	testPropagation(t, v1.MountPropagationBidirectional, "rshared")

	prop := v1.MountPropagationMode("SpaceWave")
	_, _, err := parseMountPath("/to", false, &prop)
	assert.Error(t, err)

	_, options, err := parseMountPath("/to", false, nil)
	assert.NoError(t, err)
	assert.NotContains(t, options, "private")
	assert.NotContains(t, options, "rslave")
	assert.NotContains(t, options, "rshared")
}

func TestParseMountPathRO(t *testing.T) {
	_, options, err := parseMountPath("/to", true, nil)
	assert.NoError(t, err)
	assert.Contains(t, options, "ro")

	_, options, err = parseMountPath("/to", false, nil)
	assert.NoError(t, err)
	assert.NotContains(t, options, "ro")
}

func TestGetPodPorts(t *testing.T) {
	c1 := v1.Container{
		Name: "container1",
		Ports: []v1.ContainerPort{{
			ContainerPort: 5000,
		}, {
			ContainerPort: 5001,
			HostPort:      5002,
		}},
	}
	c2 := v1.Container{
		Name: "container2",
		Ports: []v1.ContainerPort{{
			HostPort: 5004,
		}},
	}
	r := getPodPorts([]v1.Container{c1, c2}, false)
	assert.Equal(t, 2, len(r))
	assert.Equal(t, uint16(5001), r[0].ContainerPort)
	assert.Equal(t, uint16(5002), r[0].HostPort)
	assert.Equal(t, uint16(5004), r[1].ContainerPort)
	assert.Equal(t, uint16(5004), r[1].HostPort)

	r = getPodPorts([]v1.Container{c1, c2}, true)
	assert.Equal(t, 3, len(r))
	assert.Equal(t, uint16(5000), r[0].ContainerPort)
	assert.Equal(t, uint16(5000), r[0].HostPort)
	assert.Equal(t, uint16(5001), r[1].ContainerPort)
	assert.Equal(t, uint16(5002), r[1].HostPort)
	assert.Equal(t, uint16(5004), r[2].ContainerPort)
	assert.Equal(t, uint16(5004), r[2].HostPort)
}

func TestGetPortNumber(t *testing.T) {
	portSpec := intstr.IntOrString{Type: intstr.Int, IntVal: 3000, StrVal: "myport"}
	cp1 := v1.ContainerPort{Name: "myport", ContainerPort: 4000}
	cp2 := v1.ContainerPort{Name: "myport2", ContainerPort: 5000}
	i, e := getPortNumber(portSpec, []v1.ContainerPort{cp1, cp2})
	assert.NoError(t, e)
	assert.Equal(t, i, int(portSpec.IntVal))

	portSpec.Type = intstr.String
	i, e = getPortNumber(portSpec, []v1.ContainerPort{cp1, cp2})
	assert.NoError(t, e)
	assert.Equal(t, i, 4000)

	portSpec.StrVal = "not_valid"
	_, e = getPortNumber(portSpec, []v1.ContainerPort{cp1, cp2})
	assert.Error(t, e)

	portSpec.StrVal = "6000"
	i, e = getPortNumber(portSpec, []v1.ContainerPort{cp1, cp2})
	assert.NoError(t, e)
	assert.Equal(t, i, 6000)
}
