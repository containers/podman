package x86_64

import (
    `sync`
)

var (
    labelPool         sync.Pool
    programPool       sync.Pool
    instructionPool   sync.Pool
    memoryOperandPool sync.Pool
)

func freeLabel(v *Label) {
    labelPool.Put(v)
}

func clearLabel(p *Label) *Label {
    *p = Label{}
    return p
}

// CreateLabel creates a new Label, it may allocate a new one or grab one from a pool.
func CreateLabel(name string) *Label {
    var p *Label
    var v interface{}

    /* attempt to grab from the pool */
    if v = labelPool.Get(); v == nil {
        p = new(Label)
    } else {
        p = clearLabel(v.(*Label))
    }

    /* initialize the label */
    p.refs = 1
    p.Name = name
    return p
}

func newProgram(arch *Arch) *Program {
    var p *Program
    var v interface{}

    /* attempt to grab from the pool */
    if v = programPool.Get(); v == nil {
        p = new(Program)
    } else {
        p = clearProgram(v.(*Program))
    }

    /* initialize the program */
    p.arch = arch
    return p
}

func freeProgram(p *Program) {
    programPool.Put(p)
}

func clearProgram(p *Program) *Program {
    *p = Program{}
    return p
}

func newInstruction(name string, argc int, argv Operands) *Instruction {
    var v interface{}
    var p *Instruction

    /* attempt to grab from the pool */
    if v = instructionPool.Get(); v == nil {
        p = new(Instruction)
    } else {
        p = clearInstruction(v.(*Instruction))
    }

    /* initialize the instruction */
    p.name = name
    p.argc = argc
    p.argv = argv
    return p
}

func freeInstruction(v *Instruction) {
    instructionPool.Put(v)
}

func clearInstruction(p *Instruction) *Instruction {
    *p = Instruction { prefix: p.prefix[:0] }
    return p
}

func freeMemoryOperand(m *MemoryOperand) {
    memoryOperandPool.Put(m)
}

func clearMemoryOperand(m *MemoryOperand) *MemoryOperand {
    *m = MemoryOperand{}
    return m
}

// CreateMemoryOperand creates a new MemoryOperand, it may allocate a new one or grab one from a pool.
func CreateMemoryOperand() *MemoryOperand {
    var v interface{}
    var p *MemoryOperand

    /* attempt to grab from the pool */
    if v = memoryOperandPool.Get(); v == nil {
        p = new(MemoryOperand)
    } else {
        p = clearMemoryOperand(v.(*MemoryOperand))
    }

    /* initialize the memory operand */
    p.refs = 1
    return p
}
