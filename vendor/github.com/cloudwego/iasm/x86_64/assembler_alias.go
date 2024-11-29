//
// Copyright 2024 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

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
