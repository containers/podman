package generate

import (
	rspec "github.com/opencontainers/runtime-spec/specs-go"
)

func (g *Generator) initSpec() {
	if g.spec == nil {
		g.spec = &rspec.Spec{}
	}
}

func (g *Generator) initSpecProcess() {
	g.initSpec()
	if g.spec.Process == nil {
		g.spec.Process = &rspec.Process{}
	}
}

func (g *Generator) initSpecProcessConsoleSize() {
	g.initSpecProcess()
	if g.spec.Process.ConsoleSize == nil {
		g.spec.Process.ConsoleSize = &rspec.Box{}
	}
}

func (g *Generator) initSpecProcessCapabilities() {
	g.initSpecProcess()
	if g.spec.Process.Capabilities == nil {
		g.spec.Process.Capabilities = &rspec.LinuxCapabilities{}
	}
}

func (g *Generator) initSpecRoot() {
	g.initSpec()
	if g.spec.Root == nil {
		g.spec.Root = &rspec.Root{}
	}
}

func (g *Generator) initSpecAnnotations() {
	g.initSpec()
	if g.spec.Annotations == nil {
		g.spec.Annotations = make(map[string]string)
	}
}

func (g *Generator) initSpecHooks() {
	g.initSpec()
	if g.spec.Hooks == nil {
		g.spec.Hooks = &rspec.Hooks{}
	}
}

func (g *Generator) initSpecLinux() {
	g.initSpec()
	if g.spec.Linux == nil {
		g.spec.Linux = &rspec.Linux{}
	}
}

func (g *Generator) initSpecLinuxIntelRdt() {
	g.initSpecLinux()
	if g.spec.Linux.IntelRdt == nil {
		g.spec.Linux.IntelRdt = &rspec.LinuxIntelRdt{}
	}
}

func (g *Generator) initSpecLinuxSysctl() {
	g.initSpecLinux()
	if g.spec.Linux.Sysctl == nil {
		g.spec.Linux.Sysctl = make(map[string]string)
	}
}

func (g *Generator) initSpecLinuxSeccomp() {
	g.initSpecLinux()
	if g.spec.Linux.Seccomp == nil {
		g.spec.Linux.Seccomp = &rspec.LinuxSeccomp{}
	}
}

func (g *Generator) initSpecLinuxResources() {
	g.initSpecLinux()
	if g.spec.Linux.Resources == nil {
		g.spec.Linux.Resources = &rspec.LinuxResources{}
	}
}

func (g *Generator) initSpecLinuxResourcesBlockIO() {
	g.initSpecLinuxResources()
	if g.spec.Linux.Resources.BlockIO == nil {
		g.spec.Linux.Resources.BlockIO = &rspec.LinuxBlockIO{}
	}
}

func (g *Generator) initSpecLinuxResourcesCPU() {
	g.initSpecLinuxResources()
	if g.spec.Linux.Resources.CPU == nil {
		g.spec.Linux.Resources.CPU = &rspec.LinuxCPU{}
	}
}

func (g *Generator) initSpecLinuxResourcesMemory() {
	g.initSpecLinuxResources()
	if g.spec.Linux.Resources.Memory == nil {
		g.spec.Linux.Resources.Memory = &rspec.LinuxMemory{}
	}
}

func (g *Generator) initSpecLinuxResourcesNetwork() {
	g.initSpecLinuxResources()
	if g.spec.Linux.Resources.Network == nil {
		g.spec.Linux.Resources.Network = &rspec.LinuxNetwork{}
	}
}

func (g *Generator) initSpecLinuxResourcesPids() {
	g.initSpecLinuxResources()
	if g.spec.Linux.Resources.Pids == nil {
		g.spec.Linux.Resources.Pids = &rspec.LinuxPids{}
	}
}

func (g *Generator) initSpecSolaris() {
	g.initSpec()
	if g.spec.Solaris == nil {
		g.spec.Solaris = &rspec.Solaris{}
	}
}

func (g *Generator) initSpecSolarisCappedCPU() {
	g.initSpecSolaris()
	if g.spec.Solaris.CappedCPU == nil {
		g.spec.Solaris.CappedCPU = &rspec.SolarisCappedCPU{}
	}
}

func (g *Generator) initSpecSolarisCappedMemory() {
	g.initSpecSolaris()
	if g.spec.Solaris.CappedMemory == nil {
		g.spec.Solaris.CappedMemory = &rspec.SolarisCappedMemory{}
	}
}

func (g *Generator) initSpecWindows() {
	g.initSpec()
	if g.spec.Windows == nil {
		g.spec.Windows = &rspec.Windows{}
	}
}

func (g *Generator) initSpecWindowsHyperV() {
	g.initSpecWindows()
	if g.spec.Windows.HyperV == nil {
		g.spec.Windows.HyperV = &rspec.WindowsHyperV{}
	}
}

func (g *Generator) initSpecWindowsResources() {
	g.initSpecWindows()
	if g.spec.Windows.Resources == nil {
		g.spec.Windows.Resources = &rspec.WindowsResources{}
	}
}

func (g *Generator) initSpecWindowsResourcesMemory() {
	g.initSpecWindowsResources()
	if g.spec.Windows.Resources.Memory == nil {
		g.spec.Windows.Resources.Memory = &rspec.WindowsMemoryResources{}
	}
}
