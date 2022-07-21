package e2e_test

type startMachine struct {
	/*
		No command line args other than a machine vm name (also not required)
	*/
}

func (s startMachine) buildCmd(m *machineTestBuilder) []string {
	cmd := []string{"machine", "start"}
	if len(m.name) > 0 {
		cmd = append(cmd, m.name)
	}
	return cmd
}
