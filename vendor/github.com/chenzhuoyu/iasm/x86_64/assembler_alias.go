package x86_64

func alias_INT3(p *Program, vv ...interface{}) *Instruction {
    if len(vv) == 0 {
        return p.INT(3)
    } else {
        panic("instruction INT3 takes no operands")
    }
}

func alias_VCMPEQPS(p *Program, vv ...interface{}) *Instruction {
    if len(vv) >= 3 {
        return p.VCMPPS(0x00, vv[0], vv[1], vv[2], vv[3:]...)
    } else {
        panic("instruction VCMPEQPS takes 3 or 4 operands")
    }
}

func alias_VCMPTRUEPS(p *Program, vv ...interface{}) *Instruction {
    if len(vv) >= 3 {
        return p.VCMPPS(0x0f, vv[0], vv[1], vv[2], vv[3:]...)
    } else {
        panic("instruction VCMPTRUEPS takes 3 or 4 operands")
    }
}

var _InstructionAliases = map[string]_InstructionEncoder {
    "int3"       : alias_INT3,
    "retq"       : Instructions["ret"],
    "movabsq"    : Instructions["movq"],
    "vcmpeqps"   : alias_VCMPEQPS,
    "vcmptrueps" : alias_VCMPTRUEPS,
}
