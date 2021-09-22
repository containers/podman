package main

import (
	"fmt"

	"github.com/containers/podman/v3/version"
)

func main() {
	fmt.Printf(version.Version.String())
}
