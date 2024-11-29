/*
 * Copyright 2022 ByteDance Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package abi

import (
    `fmt`
    `reflect`
    `unsafe`

    . `github.com/cloudwego/iasm/x86_64`
)

const (
    PtrSize  = 8    // pointer size
    PtrAlign = 8    // pointer alignment
)

var iregOrderC = []Register{
    RDI, 
    RSI, 
    RDX, 
    RCX,
    R8, 
    R9,
}

var xregOrderC = []Register{
    XMM0,
    XMM1,
    XMM2,
    XMM3,
    XMM4,
    XMM5,
    XMM6,
    XMM7,
}

var (
    intType = reflect.TypeOf(0)
    ptrType = reflect.TypeOf(unsafe.Pointer(nil))
)

func (self *Frame) argv(i int) *MemoryOperand {
    return Ptr(RSP, int32(self.Prev() + self.desc.Args[i].Mem))
}

// spillv is used for growstack spill registers
func (self *Frame) spillv(i int) *MemoryOperand {
    // remain one slot for caller return pc
    return Ptr(RSP, PtrSize + int32(self.desc.Args[i].Mem))
}

func (self *Frame) retv(i int) *MemoryOperand {
    return Ptr(RSP, int32(self.Prev() + self.desc.Rets[i].Mem))
}

func (self *Frame) resv(i int) *MemoryOperand {
    return Ptr(RSP, int32(self.Offs() - uint32((i+1) * PtrSize)))
}

func (self *Frame) emitGrowStack(p *Program, entry *Label) {
    // spill all register arguments
    for i, v := range self.desc.Args {
        if v.InRegister {
            if v.IsFloat == floatKind64 {
                p.MOVSD(v.Reg, self.spillv(i))
            } else if v.IsFloat == floatKind32 {
                p.MOVSS(v.Reg, self.spillv(i))
            }else {
                p.MOVQ(v.Reg, self.spillv(i))
            }
        }
    }

    // call runtime.morestack_noctxt
    p.MOVQ(F_morestack_noctxt, R12)
    p.CALLQ(R12)
    // load all register arguments
    for i, v := range self.desc.Args {
        if v.InRegister {
            if v.IsFloat == floatKind64 {
                p.MOVSD(self.spillv(i), v.Reg)
            } else if v.IsFloat == floatKind32 {
                p.MOVSS(self.spillv(i), v.Reg)
            }else {
                p.MOVQ(self.spillv(i), v.Reg)
            }
        }
    }

    // jump back to the function entry
    p.JMP(entry)
}

func (self *Frame) GrowStackTextSize() uint32 {
    p := DefaultArch.CreateProgram()
    // spill all register arguments
    for i, v := range self.desc.Args {
        if v.InRegister {
            if v.IsFloat == floatKind64 {
                p.MOVSD(v.Reg, self.spillv(i))
            } else if v.IsFloat == floatKind32 {
                p.MOVSS(v.Reg, self.spillv(i))
            }else {
                p.MOVQ(v.Reg, self.spillv(i))
            }
        }
    }

    // call runtime.morestack_noctxt
    p.MOVQ(F_morestack_noctxt, R12)
    p.CALLQ(R12)
    // load all register arguments
    for i, v := range self.desc.Args {
        if v.InRegister {
            if v.IsFloat == floatKind64 {
                p.MOVSD(self.spillv(i), v.Reg)
            } else if v.IsFloat == floatKind32 {
                p.MOVSS(self.spillv(i), v.Reg)
            } else {
                p.MOVQ(self.spillv(i), v.Reg)
            }
        }
    }

    // jump back to the function entry
    l := CreateLabel("")
    p.Link(l)
    p.JMP(l)

    return uint32(len(p.Assemble(0)))
}

func (self *Frame) emitPrologue(p *Program) {
    p.SUBQ(self.Size(), RSP)
    p.MOVQ(RBP, Ptr(RSP, int32(self.Offs())))
    p.LEAQ(Ptr(RSP, int32(self.Offs())), RBP)
}

func (self *Frame) emitEpilogue(p *Program) {
    p.MOVQ(Ptr(RSP, int32(self.Offs())), RBP)
    p.ADDQ(self.Size(), RSP)
    p.RET()
}

func (self *Frame) emitReserveRegs(p *Program) {
    // spill reserved registers
    for i, r := range ReservedRegs(self.ccall) {
        switch r.(type) {
        case Register64:
            p.MOVQ(r, self.resv(i))
        case XMMRegister:
            p.MOVSD(r, self.resv(i))
        default:
            panic(fmt.Sprintf("unsupported register type %t to reserve", r))
        }
    }
}

func (self *Frame) emitSpillPtrs(p *Program) {
    // spill pointer argument registers
    for i, r := range self.desc.Args {
        if r.InRegister && r.IsPointer {
            p.MOVQ(r.Reg, self.argv(i))
        }
    }
}

func (self *Frame) emitClearPtrs(p *Program) {
    // spill pointer argument registers
    for i, r := range self.desc.Args {
        if r.InRegister && r.IsPointer {
            p.MOVQ(int64(0), self.argv(i))
        }
    }
}

func (self *Frame) emitCallC(p *Program, addr uintptr) {
    p.MOVQ(addr, RAX)
    p.CALLQ(RAX)
}

type floatKind uint8

const (
    notFloatKind floatKind = iota
    floatKind32
    floatKind64
)

type Parameter struct {
    InRegister bool
    IsPointer  bool
    IsFloat    floatKind
    Reg        Register
    Mem        uint32
    Type       reflect.Type
}

func mkIReg(vt reflect.Type, reg Register64) (p Parameter) {
    p.Reg = reg
    p.Type = vt
    p.InRegister = true
    p.IsPointer = isPointer(vt)
    return
}

func isFloat(vt reflect.Type) floatKind {
    switch vt.Kind() {
    case reflect.Float32:
        return floatKind32
    case reflect.Float64:
        return floatKind64
    default:
        return notFloatKind
    }
}

func mkXReg(vt reflect.Type, reg XMMRegister) (p Parameter) {
    p.Reg = reg
    p.Type = vt
    p.InRegister = true
    p.IsFloat = isFloat(vt)
    return
}

func mkStack(vt reflect.Type, mem uint32) (p Parameter) {
    p.Mem = mem
    p.Type = vt
    p.InRegister = false
    p.IsPointer = isPointer(vt)
    p.IsFloat = isFloat(vt)
    return
}

func (self Parameter) String() string {
    if self.InRegister {
        return fmt.Sprintf("[%%%s, Pointer(%v), Float(%v)]", self.Reg, self.IsPointer, self.IsFloat)
    } else {
        return fmt.Sprintf("[%d(FP), Pointer(%v), Float(%v)]", self.Mem, self.IsPointer, self.IsFloat)
    }
}

func CallC(addr uintptr, fr Frame, maxStack uintptr) []byte {
    p := DefaultArch.CreateProgram()

    stack := CreateLabel("_stack_grow")
    entry := CreateLabel("_entry")
    p.Link(entry)
    fr.emitStackCheck(p, stack, maxStack)
    fr.emitPrologue(p)
    fr.emitReserveRegs(p)
    fr.emitSpillPtrs(p)
    fr.emitExchangeArgs(p)
    fr.emitCallC(p, addr)
    fr.emitExchangeRets(p)
    fr.emitRestoreRegs(p)
    fr.emitEpilogue(p)
    p.Link(stack)
    fr.emitGrowStack(p, entry)

    return p.Assemble(0)
}


func (self *Frame) emitDebug(p *Program) {
    p.INT(3)
}