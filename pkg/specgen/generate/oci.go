//go:build !remote

package generate

import (
	"fmt"
	"path"
	"strings"

	"github.com/containers/common/libimage"
	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func setProcOpts(s *specgen.SpecGenerator, g *generate.Generator) {
	if s.ProcOpts == nil {
		return
	}
	for i := range g.Config.Mounts {
		if g.Config.Mounts[i].Destination == "/proc" {
			g.Config.Mounts[i].Options = s.ProcOpts
			return
		}
	}
}

func addRlimits(s *specgen.SpecGenerator, g *generate.Generator) {
	var (
		isRootless = rootless.IsRootless()
		nofileSet  = false
		nprocSet   = false
	)

	if s.Rlimits == nil {
		g.Config.Process.Rlimits = nil
		return
	}

	for _, u := range s.Rlimits {
		name := "RLIMIT_" + strings.ToUpper(u.Type)
		if name == "RLIMIT_NOFILE" {
			nofileSet = true
		} else if name == "RLIMIT_NPROC" {
			nprocSet = true
		}
		g.AddProcessRlimits(name, u.Hard, u.Soft)
	}

	// If not explicitly overridden by the user, default number of open
	// files and number of processes to the maximum they can be set to
	// (without overriding a sysctl)
	if !nofileSet {
		max := define.RLimitDefaultValue
		current := define.RLimitDefaultValue
		if isRootless {
			var rlimit unix.Rlimit
			if err := unix.Getrlimit(unix.RLIMIT_NOFILE, &rlimit); err != nil {
				logrus.Warnf("Failed to return RLIMIT_NOFILE ulimit %q", err)
			}
			if rlimit.Cur < current {
				current = rlimit.Cur
			}
			if rlimit.Max < max {
				max = rlimit.Max
			}
		}
		g.AddProcessRlimits("RLIMIT_NOFILE", max, current)
	}
	if !nprocSet {
		max := define.RLimitDefaultValue
		current := define.RLimitDefaultValue
		if isRootless {
			var rlimit unix.Rlimit
			if err := unix.Getrlimit(unix.RLIMIT_NPROC, &rlimit); err != nil {
				logrus.Warnf("Failed to return RLIMIT_NPROC ulimit %q", err)
			}
			if rlimit.Cur < current {
				current = rlimit.Cur
			}
			if rlimit.Max < max {
				max = rlimit.Max
			}
		}
		g.AddProcessRlimits("RLIMIT_NPROC", max, current)
	}
}

// Produce the final command for the container.
func makeCommand(s *specgen.SpecGenerator, imageData *libimage.ImageData, rtc *config.Config) ([]string, error) {
	finalCommand := []string{}

	entrypoint := s.Entrypoint
	if entrypoint == nil && imageData != nil {
		entrypoint = imageData.Config.Entrypoint
	}

	// Don't append the entrypoint if it is [""]
	if len(entrypoint) != 1 || entrypoint[0] != "" {
		finalCommand = append(finalCommand, entrypoint...)
	}

	// Only use image command if the user did not manually set an
	// entrypoint.
	command := s.Command
	if len(command) == 0 && imageData != nil && len(s.Entrypoint) == 0 {
		command = imageData.Config.Cmd
	}

	finalCommand = append(finalCommand, command...)

	if len(finalCommand) == 0 {
		return nil, fmt.Errorf("no command or entrypoint provided, and no CMD or ENTRYPOINT from image")
	}

	if s.Init {
		initPath := s.InitPath
		if initPath == "" && rtc != nil {
			initPath = rtc.Engine.InitPath
		}
		if initPath == "" {
			return nil, fmt.Errorf("no path to init binary found but container requested an init")
		}
		finalCommand = append([]string{define.ContainerInitPath, "--"}, finalCommand...)
	}

	return finalCommand, nil
}

// canMountSys is a best-effort heuristic to detect whether mounting a new sysfs is permitted in the container
func canMountSys(isRootless, isNewUserns bool, s *specgen.SpecGenerator) bool {
	if s.NetNS.IsHost() && (isRootless || isNewUserns) {
		return false
	}
	if isNewUserns {
		switch s.NetNS.NSMode {
		case specgen.Slirp, specgen.Private, specgen.NoNetwork, specgen.Bridge:
			return true
		default:
			return false
		}
	}
	return true
}

func getCgroupPermissons(unmask []string) string {
	ro := "ro"
	rw := "rw"
	cgroup := "/sys/fs/cgroup"

	cgroupv2, _ := cgroups.IsCgroup2UnifiedMode()
	if !cgroupv2 {
		return ro
	}

	if unmask != nil && unmask[0] == "ALL" {
		return rw
	}

	for _, p := range unmask {
		if path.Clean(p) == cgroup {
			return rw
		}
	}
	return ro
}

// SpecGenToOCI is defined in oci_linux.go for Linux-specific implementation
