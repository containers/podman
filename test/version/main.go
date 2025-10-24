package main

import (
	"fmt"

	"github.com/containers/podman/v6/version"
)

func main() {
	fmt.Print(version.Version.String())
}
