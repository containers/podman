package define

import (
	"fmt"
	"strings"
)

type VMType int64

const (
	QemuVirt VMType = iota
	WSLVirt
	AppleHvVirt
	HyperVVirt
	UnknownVirt
)

// these constants are not exported due to a conflict with the constants defined
// in define/machine_artifact.go
const (
	wsl     = "wsl"
	qemu    = "qemu"
	appleHV = "applehv"
	hyperV  = "hyperv"
)

func (v VMType) String() string {
	switch v {
	case WSLVirt:
		return wsl
	case AppleHvVirt:
		return appleHV
	case HyperVVirt:
		return hyperV
	}
	return qemu
}

func ParseVMType(input string, emptyFallback VMType) (VMType, error) {
	switch strings.TrimSpace(strings.ToLower(input)) {
	case qemu:
		return QemuVirt, nil
	case wsl:
		return WSLVirt, nil
	case appleHV:
		return AppleHvVirt, nil
	case hyperV:
		return HyperVVirt, nil
	case "":
		return emptyFallback, nil
	default:
		return UnknownVirt, fmt.Errorf("unknown VMType `%s`", input)
	}
}
