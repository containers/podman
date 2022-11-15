package e2e_test

type startMachine struct {
	/*
		No command line args other than a machine vm name (also not required)
	*/
	quiet  bool
	noInfo bool
}

func (s startMachine) buildCmd(m *machineTestBuilder) []string {
	cmd := []string{"machine", "start"}
	if len(m.name) > 0 {
		cmd = append(cmd, m.name)
	}
	if s.quiet {
		cmd = append(cmd, "--quiet")
	}
	if s.noInfo {
		cmd = append(cmd, "--no-info")
	}
	return cmd
}

func (s startMachine) withQuiet() startMachine {
	s.quiet = true
	return s
}

func (s startMachine) withNoInfo() startMachine {
	s.noInfo = true
	return s
}
