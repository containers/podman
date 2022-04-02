package e2e

type sshMachine struct {
	/*
		--username string   Username to use when ssh-ing into the VM.
	*/

	username   string
	sshCommand []string

	cmd []string
}

func (s sshMachine) buildCmd(m *machineTestBuilder) []string {
	cmd := []string{"machine", "ssh"}
	if len(m.name) > 0 {
		cmd = append(cmd, m.name)
	}
	if len(s.sshCommand) > 0 {
		cmd = append(cmd, s.sshCommand...)
	}
	return cmd
}

func (s *sshMachine) withUsername(name string) *sshMachine {
	s.username = name
	return s
}

func (s *sshMachine) withSSHComand(sshCommand []string) *sshMachine {
	s.sshCommand = sshCommand
	return s
}
