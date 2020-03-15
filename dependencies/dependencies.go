package dependencies

import (
	_ "github.com/onsi/ginkgo/ginkgo"
	_ "github.com/varlink/go/cmd/varlink-go-interface-generator" // Note: this file is used to trick `go mod` into vendoring the command below.
)
