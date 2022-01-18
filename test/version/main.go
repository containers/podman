package main

import (
	"fmt"

	"github.com/containers/podman/v4/version"
)

func main() {
	fmt.Printf(version.Version.String())
}
