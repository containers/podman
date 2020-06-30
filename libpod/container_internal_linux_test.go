// +build linux

package libpod

import (
	"io/ioutil"
	"os"
	"testing"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
)

func TestGenerateUserPasswdEntry(t *testing.T) {
	dir, err := ioutil.TempDir("", "libpod_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	c := Container{
		config: &ContainerConfig{
			User: "123:456",
			Spec: &spec.Spec{},
		},
		state: &ContainerState{
			Mountpoint: "/does/not/exist/tmp/",
		},
	}
	user, err := c.generateUserPasswdEntry()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, user, "123:x:123:456:container user:/:/bin/sh\n")

	c.config.User = "567"
	user, err = c.generateUserPasswdEntry()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, user, "567:x:567:0:container user:/:/bin/sh\n")
}
