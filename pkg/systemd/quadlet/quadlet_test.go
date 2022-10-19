package quadlet

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQuadlet_SplitPorts(t *testing.T) {
	parts := splitPorts("")
	assert.Equal(t, len(parts), 1)
	assert.Equal(t, parts[0], "")

	parts = splitPorts("foo")
	assert.Equal(t, len(parts), 1)
	assert.Equal(t, parts[0], "foo")

	parts = splitPorts("foo:bar")
	assert.Equal(t, len(parts), 2)
	assert.Equal(t, parts[0], "foo")
	assert.Equal(t, parts[1], "bar")

	parts = splitPorts("foo:bar:")
	assert.Equal(t, len(parts), 3)
	assert.Equal(t, parts[0], "foo")
	assert.Equal(t, parts[1], "bar")
	assert.Equal(t, parts[2], "")

	parts = splitPorts("abc[foo::bar]xyz:foo:bar")
	assert.Equal(t, len(parts), 3)
	assert.Equal(t, parts[0], "abc[foo::bar]xyz")
	assert.Equal(t, parts[1], "foo")
	assert.Equal(t, parts[2], "bar")

	parts = splitPorts("foo:abc[foo::bar]xyz:bar")
	assert.Equal(t, len(parts), 3)
	assert.Equal(t, parts[0], "foo")
	assert.Equal(t, parts[1], "abc[foo::bar]xyz")
	assert.Equal(t, parts[2], "bar")

	parts = splitPorts("foo:abc[foo::barxyz:bar")
	assert.Equal(t, len(parts), 2)
	assert.Equal(t, parts[0], "foo")
	assert.Equal(t, parts[1], "abc[foo::barxyz:bar")
}
