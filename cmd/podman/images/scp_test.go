package images

import (
	"testing"

	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/stretchr/testify/assert"
)

func TestParseSCPArgs(t *testing.T) {
	args := []string{"alpine", "root@localhost::"}
	var source *entities.ImageScpOptions
	var dest *entities.ImageScpOptions
	var err error
	source, _, err = parseImageSCPArg(args[0])
	assert.Nil(t, err)
	assert.Equal(t, source.Image, "alpine")

	dest, _, err = parseImageSCPArg(args[1])
	assert.Nil(t, err)
	assert.Equal(t, dest.Image, "")
	assert.Equal(t, dest.User, "root")

	args = []string{"root@localhost::alpine"}
	source, _, err = parseImageSCPArg(args[0])
	assert.Nil(t, err)
	assert.Equal(t, source.User, "root")
	assert.Equal(t, source.Image, "alpine")

	args = []string{"charliedoern@192.168.68.126::alpine", "foobar@192.168.68.126::"}
	source, _, err = parseImageSCPArg(args[0])
	assert.Nil(t, err)
	assert.True(t, source.Remote)
	assert.Equal(t, source.Image, "alpine")

	dest, _, err = parseImageSCPArg(args[1])
	assert.Nil(t, err)
	assert.True(t, dest.Remote)
	assert.Equal(t, dest.Image, "")

	args = []string{"charliedoern@192.168.68.126::alpine"}
	source, _, err = parseImageSCPArg(args[0])
	assert.Nil(t, err)
	assert.True(t, source.Remote)
	assert.Equal(t, source.Image, "alpine")
}
