//go:build linux && cgo
// +build linux,cgo

package devmapper

import jsoniter "github.com/json-iterator/go"

var json = jsoniter.ConfigCompatibleWithStandardLibrary
