package e2e

type infoMachine struct {
	format string
	cmd    []string
}

func (i *infoMachine) buildCmd(m *machineTestBuilder) []string {
	cmd := []string{"machine", "info"}
	if len(i.format) > 0 {
		cmd = append(cmd, "--format", i.format)
	}
	i.cmd = cmd
	return cmd
}

func (i *infoMachine) withFormat(format string) *infoMachine {
	i.format = format
	return i
}
