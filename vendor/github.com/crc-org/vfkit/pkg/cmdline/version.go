package cmdline

import (
	"runtime/debug"
	"strings"
)

var (
	// set using the '-X github.com/crc-org/vfkit/pkg/cmdline.gitVersion' linker flag
	gitVersion = "unknown"

	// set through .gitattributes when `git archive` is used
	// see https://icinga.com/blog/2022/05/25/embedding-git-commit-information-in-go-binaries/
	gitArchiveVersion = "$Format:%(describe)$"
)

func Version() string {
	switch {
	// This will be substituted when building from a GitHub tarball
	case !strings.HasPrefix(gitArchiveVersion, "$Format:"):
		return gitArchiveVersion
	// This will be set when building from git using make
	case gitVersion != "":
		return gitVersion
	// moduleVersionFromBuildInfo() will be set when using `go install`
	default:
		return moduleVersionFromBuildInfo()
	}
}

func moduleVersionFromBuildInfo() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	if info.Main.Version == "(devel)" {
		return ""
	}
	return info.Main.Version
}
