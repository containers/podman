package e2e

type basicMachine struct {
	args []string
	cmd  []string
}

func (s basicMachine) buildCmd(m *machineTestBuilder) []string {
	cmd := []string{"-r"}
	if len(s.args) > 0 {
		cmd = append(cmd, s.args...)
	}
	return cmd
}

func (s *basicMachine) withPodmanCommand(args []string) *basicMachine {
	s.args = args
	return s
}
