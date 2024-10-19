//go:build !windows && !darwin
// +build !windows,!darwin

package chrootarchive

import jsoniter "github.com/json-iterator/go"

var json = jsoniter.ConfigCompatibleWithStandardLibrary
