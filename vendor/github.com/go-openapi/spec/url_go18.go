//go:build !go1.19
// +build !go1.19

package spec

import "net/url"

var parseURL = url.Parse
