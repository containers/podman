package main

import (
	"fmt"

	"github.com/containers/podman/v5/version"
)

func main() {
	fmt.Print(version.Version.String())
}
