package e2e_test

import "os"

const podmanBinary = "../../../bin/podman-remote"

var fakeImagePath string = os.DevNull

func (i *initMachine) withFakeImage(_ *machineTestBuilder) *initMachine {
	i.image = fakeImagePath
	return i
}
