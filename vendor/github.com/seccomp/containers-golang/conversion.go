package seccomp // import "github.com/seccomp/containers-golang"

import "fmt"

var goArchToSeccompArchMap = map[string]Arch{
	"386":         ArchX86,
	"amd64":       ArchX86_64,
	"amd64p32":    ArchX32,
	"arm":         ArchARM,
	"arm64":       ArchAARCH64,
	"mips":        ArchMIPS,
	"mips64":      ArchMIPS64,
	"mips64le":    ArchMIPSEL64,
	"mips64p32":   ArchMIPS64N32,
	"mips64p32le": ArchMIPSEL64N32,
	"mipsle":      ArchMIPSEL,
	"ppc":         ArchPPC,
	"ppc64":       ArchPPC64,
	"ppc64le":     ArchPPC64LE,
	"s390":        ArchS390,
	"s390x":       ArchS390X,
}

// GoArchToSeccompArch converts a runtime.GOARCH to a seccomp `Arch`. The
// function returns an error if the architecture conversion is not supported.
func GoArchToSeccompArch(goArch string) (Arch, error) {
	arch, ok := goArchToSeccompArchMap[goArch]
	if !ok {
		return "", fmt.Errorf("unsupported go arch provided: %s", goArch)
	}
	return arch, nil
}
