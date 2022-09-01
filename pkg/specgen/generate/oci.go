package generate

import (
	"fmt"
	"strings"

	"github.com/containers/common/libimage"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

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
		max := rlimT(define.RLimitDefaultValue)
		current := rlimT(define.RLimitDefaultValue)
		if isRootless {
			var rlimit unix.Rlimit
			if err := unix.Getrlimit(unix.RLIMIT_NOFILE, &rlimit); err != nil {
				logrus.Warnf("Failed to return RLIMIT_NOFILE ulimit %q", err)
			}
			if rlimT(rlimit.Cur) < current {
				current = rlimT(rlimit.Cur)
			}
			if rlimT(rlimit.Max) < max {
				max = rlimT(rlimit.Max)
			}
		}
		g.AddProcessRlimits("RLIMIT_NOFILE", uint64(max), uint64(current))
	}
	if !nprocSet {
		max := rlimT(define.RLimitDefaultValue)
		current := rlimT(define.RLimitDefaultValue)
		if isRootless {
			var rlimit unix.Rlimit
			if err := unix.Getrlimit(unix.RLIMIT_NPROC, &rlimit); err != nil {
				logrus.Warnf("Failed to return RLIMIT_NPROC ulimit %q", err)
			}
			if rlimT(rlimit.Cur) < current {
				current = rlimT(rlimit.Cur)
			}
			if rlimT(rlimit.Max) < max {
				max = rlimT(rlimit.Max)
			}
		}
		g.AddProcessRlimits("RLIMIT_NPROC", uint64(max), uint64(current))
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
