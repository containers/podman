package x86_64

// CreateLabel creates a new Label, it may allocate a new one or grab one from a pool.
func CreateLabel(name string) *Label {
	p := new(Label)

	/* initialize the label */
	p.refs = 1
	p.Name = name
	return p
}

func newProgram(arch *Arch) *Program {
	p := new(Program)

	/* initialize the program */
	p.arch = arch
	return p
}

func newInstruction(name string, argc int, argv Operands) *Instruction {
	p := new(Instruction)

	/* initialize the instruction */
	p.name = name
	p.argc = argc
	p.argv = argv
	return p
}

// CreateMemoryOperand creates a new MemoryOperand, it may allocate a new one or grab one from a pool.
func CreateMemoryOperand() *MemoryOperand {
	p := new(MemoryOperand)

	/* initialize the memory operand */
	p.refs = 1
	return p
}
