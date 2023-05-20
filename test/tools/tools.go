//go:build tools
// +build tools

package tools

// Importing the packages here will allow to vendor those via
// `go mod vendor`.

import (
	_ "github.com/cpuguy83/go-md2man/v2"
	_ "github.com/onsi/ginkgo/v2/ginkgo"
	_ "github.com/vbatts/git-validation"
	_ "golang.org/x/tools/cmd/goimports"
)
