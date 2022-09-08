package specgen

import (
	"github.com/containers/common/pkg/config"
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

func (s *SpecGenerator) InitResourceLimits(rtc *config.Config) {
	if s.ResourceLimits == nil || s.ResourceLimits.Pids == nil {
		if s.CgroupsMode != "disabled" {
			limit := rtc.PidsLimit()
			if limit != 0 {
				if s.ResourceLimits == nil {
					s.ResourceLimits = &spec.LinuxResources{}
				}
				s.ResourceLimits.Pids = &spec.LinuxPids{
					Limit: limit,
				}
			}
		}
	}
}
