package x86_64

import (
    `bytes`
    `errors`
    `fmt`
    `math`
    `strconv`
    `strings`
    `unicode`

    `github.com/chenzhuoyu/iasm/expr`
)

type (
    _TokenKind   int
    _Punctuation int
)

const (
    _T_end _TokenKind = iota + 1
    _T_int
    _T_name
    _T_punc
    _T_space
)

const (
    _P_plus _Punctuation = iota + 1
    _P_minus
    _P_star
    _P_slash
    _P_percent
    _P_amp
    _P_bar
    _P_caret
    _P_shl
    _P_shr
    _P_tilde
    _P_lbrk
    _P_rbrk
    _P_dot
    _P_comma
    _P_colon
    _P_dollar
    _P_hash
)

var _PUNC_NAME = map[_Punctuation]string {
    _P_plus    : "+",
    _P_minus   : "-",
    _P_star    : "*",
    _P_slash   : "/",
    _P_percent : "%",
    _P_amp     : "&",
    _P_bar     : "|",
    _P_caret   : "^",
    _P_shl     : "<<",
    _P_shr     : ">>",
    _P_tilde   : "~",
    _P_lbrk    : "(",
    _P_rbrk    : ")",
    _P_dot     : ".",
    _P_comma   : ",",
    _P_colon   : ":",
    _P_dollar  : "$",
    _P_hash    : "#",
}

func (self _Punctuation) String() string {
    if v, ok := _PUNC_NAME[self]; ok {
        return v
    } else {
        return fmt.Sprintf("_Punctuation(%d)", self)
    }
}

type _Token struct {
    pos int
    end int
    u64 uint64
    str string
    tag _TokenKind
}

func (self *_Token) punc() _Punctuation {
    return _Punctuation(self.u64)
}

func (self *_Token) String() string {
    switch self.tag {
        case _T_end   : return "<END>"
        case _T_int   : return fmt.Sprintf("<INT %d>", self.u64)
        case _T_punc  : return fmt.Sprintf("<PUNC %s>", _Punctuation(self.u64))
        case _T_name  : return fmt.Sprintf("<NAME %s>", strconv.QuoteToASCII(self.str))
        case _T_space : return "<SPACE>"
        default       : return fmt.Sprintf("<UNK:%d %d %s>", self.tag, self.u64, strconv.QuoteToASCII(self.str))
    }
}

func tokenEnd(p int, end int) _Token {
    return _Token {
        pos: p,
        end: end,
        tag: _T_end,
    }
}

func tokenInt(p int, val uint64) _Token {
    return _Token {
        pos: p,
        u64: val,
        tag: _T_int,
    }
}

func tokenName(p int, name string) _Token {
    return _Token {
        pos: p,
        str: name,
        tag: _T_name,
    }
}

func tokenPunc(p int, punc _Punctuation) _Token {
    return _Token {
        pos: p,
        tag: _T_punc,
        u64: uint64(punc),
    }
}

func tokenSpace(p int, end int) _Token {
    return _Token {
        pos: p,
        end: end,
        tag: _T_space,
    }
}

// SyntaxError represents an error in the assembly syntax.
type SyntaxError struct {
    Pos    int
    Row    int
    Src    []rune
    Reason string
}

// Error implements the error interface.
func (self *SyntaxError) Error() string {
    if self.Pos < 0 {
        return fmt.Sprintf("%s at line %d", self.Reason, self.Row)
    } else {
        return fmt.Sprintf("%s at %d:%d", self.Reason, self.Row, self.Pos + 1)
    }
}

type _Tokenizer struct {
    pos int
    row int
    src []rune
}

func (self *_Tokenizer) ch() rune {
    return self.src[self.pos]
}

func (self *_Tokenizer) eof() bool {
    return self.pos >= len(self.src)
}

func (self *_Tokenizer) rch() (ret rune) {
    ret, self.pos = self.src[self.pos], self.pos + 1
    return
}

func (self *_Tokenizer) err(pos int, msg string) *SyntaxError {
    return &SyntaxError {
        Pos    : pos,
        Row    : self.row,
        Src    : self.src,
        Reason : msg,
    }
}

type _TrimState int

const (
    _TS_normal _TrimState = iota
    _TS_slcomm
    _TS_hscomm
    _TS_string
    _TS_escape
    _TS_accept
    _TS_nolast
)

func (self *_Tokenizer) init(src string) {
    var i int
    var ch rune
    var st _TrimState

    /* set the source */
    self.pos = 0
    self.src = []rune(src)

    /* remove commends, including "//" and "##" */
    loop: for i, ch = range self.src {
        switch {
            case st == _TS_normal && ch == '/'  : st = _TS_slcomm
            case st == _TS_normal && ch == '"'  : st = _TS_string
            case st == _TS_normal && ch == ';'  : st = _TS_accept; break loop
            case st == _TS_normal && ch == '#'  : st = _TS_hscomm
            case st == _TS_slcomm && ch == '/'  : st = _TS_nolast; break loop
            case st == _TS_slcomm               : st = _TS_normal
            case st == _TS_hscomm && ch == '#'  : st = _TS_nolast; break loop
            case st == _TS_hscomm               : st = _TS_normal
            case st == _TS_string && ch == '"'  : st = _TS_normal
            case st == _TS_string && ch == '\\' : st = _TS_escape
            case st == _TS_escape               : st = _TS_string
        }
    }

    /* check for errors */
    switch st {
        case _TS_accept: self.src = self.src[:i]
        case _TS_nolast: self.src = self.src[:i - 1]
        case _TS_string: panic(self.err(i, "string is not terminated"))
        case _TS_escape: panic(self.err(i, "escape sequence is not terminated"))
    }
}

func (self *_Tokenizer) skip(check func(v rune) bool) {
    for !self.eof() && check(self.ch()) {
        self.pos++
    }
}

func (self *_Tokenizer) find(pos int, check func(v rune) bool) string {
    self.skip(check)
    return string(self.src[pos:self.pos])
}

func (self *_Tokenizer) chrv(p int) _Token {
    var err error
    var val uint64

    /* starting and ending position */
    p0 := p + 1
    p1 := p0 + 1

    /* find the end of the literal */
    for p1 < len(self.src) && self.src[p1] != '\'' {
        if p1++; self.src[p1 - 1] == '\\' {
            p1++
        }
    }

    /* empty literal */
    if p1 == p0 {
        panic(self.err(p1, "empty character constant"))
    }

    /* check for EOF */
    if p1 == len(self.src) {
        panic(self.err(p1, "unexpected EOF when scanning literals"))
    }

    /* parse the literal */
    if val, err = literal64(string(self.src[p0:p1])); err != nil {
        panic(self.err(p0, "cannot parse literal: " + err.Error()))
    }

    /* skip the closing '\'' */
    self.pos = p1 + 1
    return tokenInt(p, val)
}

func (self *_Tokenizer) numv(p int) _Token {
    if val, err := strconv.ParseUint(self.find(p, isnumber), 0, 64); err != nil {
        panic(self.err(p, "invalid immediate value: " + err.Error()))
    } else {
        return tokenInt(p, val)
    }
}

func (self *_Tokenizer) defv(p int, cc rune) _Token {
    if isdigit(cc) {
        return self.numv(p)
    } else if isident0(cc) {
        return tokenName(p, self.find(p, isident))
    } else {
        panic(self.err(p, "invalid char: " + strconv.QuoteRune(cc)))
    }
}

func (self *_Tokenizer) rep2(p int, pp _Punctuation, cc rune) _Token {
    if self.eof() {
        panic(self.err(self.pos, "unexpected EOF when scanning operators"))
    } else if c := self.rch(); c != cc {
        panic(self.err(p + 1, strconv.QuoteRune(cc) + " expected, got " + strconv.QuoteRune(c)))
    } else {
        return tokenPunc(p, pp)
    }
}

func (self *_Tokenizer) read() _Token {
    var p int
    var c rune
    var t _Token

    /* check for EOF */
    if self.eof() {
        return tokenEnd(self.pos, self.pos)
    }

    /* skip spaces as needed */
    if p = self.pos; unicode.IsSpace(self.src[p]) {
        self.skip(unicode.IsSpace)
        return tokenSpace(p, self.pos)
    }

    /* check for line comments */
    if p = self.pos; p < len(self.src) - 1 && self.src[p] == '/' && self.src[p + 1] == '/' {
        self.pos = len(self.src)
        return tokenEnd(p, self.pos)
    }

    /* read the next character */
    p = self.pos
    c = self.rch()

    /* parse the next character */
    switch c {
        case '+'  : t = tokenPunc(p, _P_plus)
        case '-'  : t = tokenPunc(p, _P_minus)
        case '*'  : t = tokenPunc(p, _P_star)
        case '/'  : t = tokenPunc(p, _P_slash)
        case '%'  : t = tokenPunc(p, _P_percent)
        case '&'  : t = tokenPunc(p, _P_amp)
        case '|'  : t = tokenPunc(p, _P_bar)
        case '^'  : t = tokenPunc(p, _P_caret)
        case '<'  : t = self.rep2(p, _P_shl, '<')
        case '>'  : t = self.rep2(p, _P_shr, '>')
        case '~'  : t = tokenPunc(p, _P_tilde)
        case '('  : t = tokenPunc(p, _P_lbrk)
        case ')'  : t = tokenPunc(p, _P_rbrk)
        case '.'  : t = tokenPunc(p, _P_dot)
        case ','  : t = tokenPunc(p, _P_comma)
        case ':'  : t = tokenPunc(p, _P_colon)
        case '$'  : t = tokenPunc(p, _P_dollar)
        case '#'  : t = tokenPunc(p, _P_hash)
        case '\'' : t = self.chrv(p)
        default   : t = self.defv(p, c)
    }

    /* mark the end of token */
    t.end = self.pos
    return t
}

func (self *_Tokenizer) next() (tk _Token) {
    for {
        if tk = self.read(); tk.tag != _T_space {
            return
        }
    }
}

// LabelKind indicates the type of label reference.
type LabelKind int

// OperandKind indicates the type of the operand.
type OperandKind int

// InstructionPrefix indicates the prefix bytes prepended to the instruction.
type InstructionPrefix byte

const (
    // OpImm means the operand is an immediate value.
    OpImm OperandKind = 1 << iota

    // OpReg means the operand is a register.
    OpReg

    // OpMem means the operand is a memory address.
    OpMem

    // OpLabel means the operand is a label, specifically for
    // branch instructions.
    OpLabel
)

const (
    // Declaration means the label is a declaration.
    Declaration LabelKind = iota + 1

    // BranchTarget means the label should be treated as a branch target.
    BranchTarget

    // RelativeAddress means the label should be treated as a reference to
    // the code section (e.g. RIP-relative addressing).
    RelativeAddress
)

const (
    // PrefixLock causes the processor's LOCK# signal to be asserted during execution of
    // the accompanying instruction (turns the instruction into an atomic instruction).
    // In a multiprocessor environment, the LOCK# signal insures that the processor
    // has exclusive use of any shared memory while the signal is asserted.
    PrefixLock InstructionPrefix = iota

    // PrefixSegmentCS overrides the memory operation of this instruction to CS (Code Segment).
    PrefixSegmentCS

    // PrefixSegmentDS overrides the memory operation of this instruction to DS (Data Segment),
    // this is the default section for most instructions if not specified.
    PrefixSegmentDS

    // PrefixSegmentES overrides the memory operation of this instruction to ES (Extra Segment).
    PrefixSegmentES

    // PrefixSegmentFS overrides the memory operation of this instruction to FS.
    PrefixSegmentFS

    // PrefixSegmentGS overrides the memory operation of this instruction to GS.
    PrefixSegmentGS

    // PrefixSegmentSS overrides the memory operation of this instruction to SS (Stack Segment).
    PrefixSegmentSS
)

// ParsedLabel represents a label in the source, either a jump target or
// an RIP-relative addressing.
type ParsedLabel struct {
    Name string
    Kind LabelKind
}

// ParsedOperand represents an operand of an instruction in the source.
type ParsedOperand struct {
    Op     OperandKind
    Imm    int64
    Reg    Register
    Label  ParsedLabel
    Memory MemoryAddress
}

// ParsedInstruction represents an instruction in the source.
type ParsedInstruction struct {
    Mnemonic string
    Operands []ParsedOperand
    Prefixes []InstructionPrefix
}

func (self *ParsedInstruction) imm(v int64) {
    self.Operands = append(self.Operands, ParsedOperand {
        Op  : OpImm,
        Imm : v,
    })
}

func (self *ParsedInstruction) reg(v Register) {
    self.Operands = append(self.Operands, ParsedOperand {
        Op  : OpReg,
        Reg : v,
    })
}

func (self *ParsedInstruction) mem(v MemoryAddress) {
    self.Operands = append(self.Operands, ParsedOperand {
        Op     : OpMem,
        Memory : v,
    })
}

func (self *ParsedInstruction) target(v string) {
    self.Operands = append(self.Operands, ParsedOperand {
        Op    : OpLabel,
        Label : ParsedLabel {
            Name: v,
            Kind: BranchTarget,
        },
    })
}

func (self *ParsedInstruction) reference(v string) {
    self.Operands = append(self.Operands, ParsedOperand {
        Op    : OpLabel,
        Label : ParsedLabel {
            Name: v,
            Kind: RelativeAddress,
        },
    })
}

// LineKind indicates the type of ParsedLine.
type LineKind int

const (
    // LineLabel means the ParsedLine is a label.
    LineLabel LineKind = iota + 1

    // LineInstr means the ParsedLine is an instruction.
    LineInstr

    // LineCommand means the ParsedLine is a ParsedCommand.
    LineCommand
)

// ParsedLine represents a parsed source line.
type ParsedLine struct {
    Row         int
    Src         []rune
    Kind        LineKind
    Label       ParsedLabel
    Command     ParsedCommand
    Instruction ParsedInstruction
}

// ParsedCommand represents a parsed assembly directive command.
type ParsedCommand struct {
    Cmd  string
    Args []ParsedCommandArg
}

// ParsedCommandArg represents an argument of a ParsedCommand.
type ParsedCommandArg struct {
    Value    string
    IsString bool
}

// Parser parses the source, and generates a sequence of ParsedInstruction's.
type Parser struct {
    lex _Tokenizer
    exp expr.Parser
}

const (
    rip Register64 = 0xff
)

var _RegBranch = map[string]bool {
    "jmp"   : true,
    "jmpq"  : true,
    "call"  : true,
    "callq" : true,
}

var _SegPrefix = map[string]InstructionPrefix {
    "cs": PrefixSegmentCS,
    "ds": PrefixSegmentDS,
    "es": PrefixSegmentES,
    "fs": PrefixSegmentFS,
    "gs": PrefixSegmentGS,
    "ss": PrefixSegmentSS,
}

func (self *Parser) i32(tk _Token, v int64) int32 {
    if v >= math.MinInt32 && v <= math.MaxUint32 {
        return int32(v)
    } else {
        panic(self.err(tk.pos, fmt.Sprintf("32-bit integer out ouf range: %d", v)))
    }
}

func (self *Parser) err(pos int, msg string) *SyntaxError {
    return &SyntaxError {
        Pos    : pos,
        Row    : self.lex.row,
        Src    : self.lex.src,
        Reason : msg,
    }
}

func (self *Parser) negv() int64 {
    tk := self.lex.read()
    tt := tk.tag

    /* must be an integer */
    if tt != _T_int {
        panic(self.err(tk.pos, "integer expected after '-'"))
    } else {
        return -int64(tk.u64)
    }
}

func (self *Parser) eval(p int) (r int64) {
    var e error
    var v *expr.Expr

    /* searching start */
    n := 1
    q := p + 1

    /* find the end of expression */
    for n > 0 && q < len(self.lex.src) {
        switch self.lex.src[q] {
            case '(' : q++; n++
            case ')' : q++; n--
            default  : q++
        }
    }

    /* check for EOF */
    if n != 0 {
        panic(self.err(q, "unexpected EOF when parsing expressions"))
    }

    /* evaluate the expression */
    if v, e = self.exp.SetSource(string(self.lex.src[p:q - 1])).Parse(nil); e != nil {
        panic(self.err(p, "cannot evaluate expression: " + e.Error()))
    }

    /* evaluate the expression */
    if r, e = v.Evaluate(); e != nil {
        panic(self.err(p, "cannot evaluate expression: " + e.Error()))
    }

    /* skip the last ')' */
    v.Free()
    self.lex.pos = q
    return
}

func (self *Parser) relx(tk _Token) {
    if tk.tag != _T_punc || tk.punc() != _P_lbrk {
        panic(self.err(tk.pos, "'(' expected for RIP-relative addressing"))
    } else if tk = self.lex.next(); self.regx(tk) != rip {
        panic(self.err(tk.pos, "RIP-relative addressing expects %rip as the base register"))
    } else if tk = self.lex.next(); tk.tag != _T_punc || tk.punc() != _P_rbrk {
        panic(self.err(tk.pos, "RIP-relative addressing does not support indexing or scaling"))
    }
}

func (self *Parser) immx(tk _Token) int64 {
    if tk.tag != _T_punc || tk.punc() != _P_dollar {
        panic(self.err(tk.pos, "'$' expected for registers"))
    } else if tk = self.lex.read(); tk.tag == _T_int {
        return int64(tk.u64)
    } else if tk.tag == _T_punc && tk.punc() == _P_lbrk {
        return self.eval(self.lex.pos)
    } else if tk.tag == _T_punc && tk.punc() == _P_minus {
        return self.negv()
    } else {
        panic(self.err(tk.pos, "immediate value expected"))
    }
}

func (self *Parser) regx(tk _Token) Register {
    if tk.tag != _T_punc || tk.punc() != _P_percent {
        panic(self.err(tk.pos, "'%' expected for registers"))
    } else if tk = self.lex.read(); tk.tag != _T_name {
        panic(self.err(tk.pos, "register name expected"))
    } else if tk.str == "rip" {
        return rip
    } else if reg, ok := Registers[tk.str]; ok {
        return reg
    } else {
        panic(self.err(tk.pos, "invalid register name: " + strconv.Quote(tk.str)))
    }
}

func (self *Parser) regv(tk _Token) Register {
    if reg := self.regx(tk); reg == rip {
        panic(self.err(tk.pos, "%rip is not accessable as a dedicated register"))
    } else {
        return reg
    }
}

func (self *Parser) disp(vv int32) MemoryAddress {
    switch tk := self.lex.next(); tk.tag {
        case _T_end  : return MemoryAddress { Displacement: vv }
        case _T_punc : return self.relm(tk, vv)
        default      : panic(self.err(tk.pos, "',' or '(' expected"))
    }
}

func (self *Parser) relm(tv _Token, disp int32) MemoryAddress {
    var tk _Token
    var tt _TokenKind

    /* check for absolute addressing */
    if tv.punc() == _P_comma {
        self.lex.pos--
        return MemoryAddress { Displacement: disp }
    }

    /* must be '(' now */
    if tv.punc() != _P_lbrk {
        panic(self.err(tv.pos, "',' or '(' expected"))
    }

    /* read the next token */
    tk = self.lex.next()
    tt = tk.tag

    /* must be a punctuation */
    if tt != _T_punc {
        panic(self.err(tk.pos, "'%' or ',' expected"))
    }

    /* check for base */
    switch tk.punc() {
        case _P_percent : return self.base(tk, disp)
        case _P_comma   : return self.index(nil, disp)
        default         : panic(self.err(tk.pos, "'%' or ',' expected"))
    }
}

func (self *Parser) base(tk _Token, disp int32) MemoryAddress {
    rr := self.regx(tk)
    nk := self.lex.next()

    /* check for register indirection or base-index addressing */
    if !isReg64(rr) {
        panic(self.err(tk.pos, "not a valid base register"))
    } else if nk.tag != _T_punc {
        panic(self.err(nk.pos, "',' or ')' expected"))
    } else if nk.punc() == _P_comma {
        return self.index(rr, disp)
    } else if nk.punc() == _P_rbrk {
        return MemoryAddress { Base: rr, Displacement: disp }
    } else {
        panic(self.err(nk.pos, "',' or ')' expected"))
    }
}

func (self *Parser) index(base Register, disp int32) MemoryAddress {
    tk := self.lex.next()
    rr := self.regx(tk)
    nk := self.lex.next()

    /* check for scaled indexing */
    if base == rip {
        panic(self.err(tk.pos, "RIP-relative addressing does not support indexing or scaling"))
    } else if !isIndexable(rr) {
        panic(self.err(tk.pos, "not a valid index register"))
    } else if nk.tag != _T_punc {
        panic(self.err(nk.pos, "',' or ')' expected"))
    } else if nk.punc() == _P_comma {
        return self.scale(base, rr, disp)
    } else if nk.punc() == _P_rbrk {
        return MemoryAddress { Base: base, Index: rr, Scale: 1, Displacement: disp }
    } else {
        panic(self.err(nk.pos, "',' or ')' expected"))
    }
}

func (self *Parser) scale(base Register, index Register, disp int32) MemoryAddress {
    tk := self.lex.next()
    tt := tk.tag
    tv := tk.u64

    /* must be an integer */
    if tt != _T_int {
        panic(self.err(tk.pos, "integer expected"))
    }

    /* scale can only be 1, 2, 4 or 8 */
    if tv == 0 || (_Scales & (1 << tv)) == 0 {
        panic(self.err(tk.pos, "scale can only be 1, 2, 4 or 8"))
    }

    /* read next token */
    tk = self.lex.next()
    tt = tk.tag

    /* check for the closing ')' */
    if tt != _T_punc || tk.punc() != _P_rbrk {
        panic(self.err(tk.pos, "')' expected"))
    }

    /* construct the memory address */
    return MemoryAddress {
        Base         : base,
        Index        : index,
        Scale        : uint8(tv),
        Displacement : disp,
    }
}

func (self *Parser) cmds() *ParsedLine {
    cmd := ""
    pos := self.lex.pos
    buf := []ParsedCommandArg(nil)

    /* find the end of command */
    for p := pos; pos < len(self.lex.src); pos++ {
        if unicode.IsSpace(self.lex.src[pos]) {
            cmd = string(self.lex.src[p:pos])
            break
        }
    }

    /* parse the arguments */
    loop: for {
        switch self.next(&pos) {
            case 0   : break loop
            case '#' : break loop
            case '"' : pos = self.strings(&buf, pos)
            default  : pos = self.expressions(&buf, pos)
        }
    }

    /* construct the line */
    return &ParsedLine {
        Row     : self.lex.row,
        Src     : self.lex.src,
        Kind    : LineCommand,
        Command : ParsedCommand {
            Cmd  : cmd,
            Args : buf,
        },
    }
}

func (self *Parser) feed(line string) *ParsedLine {
    ff := true
    rr := false
    lk := false

    /* reset the lexer */
    self.lex.row++
    self.lex.init(line)

    /* parse the first token */
    tk := self.lex.next()
    tt := tk.tag

    /* it is a directive if it starts with a dot */
    if tk.tag == _T_punc && tk.punc() == _P_dot {
        return self.cmds()
    }

    /* otherwise it could be labels or instructions */
    if tt != _T_name {
        panic(self.err(tk.pos, "identifier expected"))
    }

    /* peek the next token */
    lex := self.lex
    tkx := lex.next()

    /* check for labels */
    if tkx.tag == _T_punc && tkx.punc() == _P_colon {
        tkx = lex.next()
        ttx := tkx.tag

        /* the line must end here */
        if ttx != _T_end {
            panic(self.err(tkx.pos, "garbage after label definition"))
        }

        /* construct the label */
        return &ParsedLine {
            Row   : self.lex.row,
            Src   : self.lex.src,
            Kind  : LineLabel,
            Label : ParsedLabel {
                Kind: Declaration,
                Name: tk.str,
            },
        }
    }

    /* special case for the "lock" prefix */
    if tk.tag == _T_name && strings.ToLower(tk.str) == "lock" {
        lk = true
        tk = self.lex.next()

        /* must be an instruction */
        if tk.tag != _T_name {
            panic(self.err(tk.pos, "identifier expected"))
        }
    }

    /* set the line kind and mnemonic */
    ret := &ParsedLine {
        Row         : self.lex.row,
        Src         : self.lex.src,
        Kind        : LineInstr,
        Instruction : ParsedInstruction { Mnemonic: strings.ToLower(tk.str) },
    }

    /* check for LOCK prefix */
    if lk {
        ret.Instruction.Prefixes = append(ret.Instruction.Prefixes, PrefixLock)
    }

    /* parse all the operands */
    for {
        tk = self.lex.next()
        tt = tk.tag

        /* check for end of line */
        if tt == _T_end {
            break
        }

        /* expect a comma if not the first operand */
        if !ff {
            if tt == _T_punc && tk.punc() == _P_comma {
                tk = self.lex.next()
            } else {
                panic(self.err(tk.pos, "',' expected"))
            }
        }

        /* not the first operand anymore */
        ff = false
        tt = tk.tag

        /* encountered an integer, must be a SIB memory address */
        if tt == _T_int {
            ret.Instruction.mem(self.disp(self.i32(tk, int64(tk.u64))))
            continue
        }

        /* encountered an identifier, maybe an expression or a jump target, or a segment override prefix */
        if tt == _T_name {
            ts := tk.str
            tp := self.lex.pos

            /* if the next token is EOF or a comma, it's a jumpt target */
            if tk = self.lex.next(); tk.tag == _T_end || (tk.tag == _T_punc && tk.punc() == _P_comma) {
                self.lex.pos = tp
                ret.Instruction.target(ts)
                continue
            }

            /* if it is a colon, it's a segment override prefix, otherwise it must be an RIP-relative addressing operand */
            if tk.tag != _T_punc || tk.punc() != _P_colon {
                self.relx(tk)
                ret.Instruction.reference(ts)
                continue
            }

            /* lookup segment prefixes */
            if p, ok := _SegPrefix[strings.ToLower(ts)]; !ok {
                panic(self.err(tk.pos, "invalid segment name"))
            } else {
                ret.Instruction.Prefixes = append(ret.Instruction.Prefixes, p)
            }

            /* read the next token */
            tk = self.lex.next()
            tt = tk.tag

            /* encountered an integer, must be a SIB memory address */
            if tt == _T_int {
                ret.Instruction.mem(self.disp(self.i32(tk, int64(tk.u64))))
                continue
            }
        }

        /* certain instructions may have a "*" before operands */
        if tt == _T_punc && tk.punc() == _P_star {
            tk = self.lex.next()
            tt = tk.tag
            rr = true
        }

        /* ... otherwise it must be a punctuation */
        if tt != _T_punc {
            panic(self.err(tk.pos, "'$', '%', '-' or '(' expected"))
        }

        /* check the operator */
        switch tk.punc() {
            case _P_lbrk    : break
            case _P_minus   : ret.Instruction.mem(self.disp(self.i32(tk, self.negv()))) ; continue
            case _P_dollar  : ret.Instruction.imm(self.immx(tk))                        ; continue
            case _P_percent : ret.Instruction.reg(self.regv(tk))                        ; continue
            default         : panic(self.err(tk.pos, "'$', '%', '-' or '(' expected"))
        }

        /* special case of '(', might be either `(expr)(SIB)` or just `(SIB)`
         * read one more token to confirm */
        tk = self.lex.next()
        tt = tk.tag

        /* the next token is '%', it's a memory address,
         * or ',' if it's a memory address without base,
         * otherwise it must be in `(expr)(SIB)` form */
        if tk.tag == _T_punc && tk.punc() == _P_percent {
            ret.Instruction.mem(self.base(tk, 0))
        } else if tk.tag == _T_punc && tk.punc() == _P_comma {
            ret.Instruction.mem(self.index(nil, 0))
        } else {
            ret.Instruction.mem(self.disp(self.i32(tk, self.eval(tk.pos))))
        }
    }

    /* check "jmp" and "call" instructions */
    if !_RegBranch[ret.Instruction.Mnemonic] {
        return ret
    } else if len(ret.Instruction.Operands) != 1 {
        panic(self.err(tk.pos, fmt.Sprintf(`"%s" requires exact 1 argument`, ret.Instruction.Mnemonic)))
    } else if !rr && ret.Instruction.Operands[0].Op != OpReg && ret.Instruction.Operands[0].Op != OpLabel {
        panic(self.err(tk.pos, fmt.Sprintf(`invalid operand for "%s" instruction`, ret.Instruction.Mnemonic)))
    } else {
        return ret
    }
}

func (self *Parser) next(p *int) rune {
    for {
        if *p >= len(self.lex.src) {
            return 0
        } else if cc := self.lex.src[*p]; !unicode.IsSpace(cc) {
            return cc
        } else {
            *p++
        }
    }
}

func (self *Parser) delim(p int) int {
    if cc := self.next(&p); cc == 0 {
        return p
    } else if cc == ',' {
        return p + 1
    } else {
        panic(self.err(p, "',' expected"))
    }
}

func (self *Parser) strings(argv *[]ParsedCommandArg, p int) int {
    var i int
    var e error
    var v string

    /* find the end of string */
    for i = p + 1; i < len(self.lex.src) && self.lex.src[i] != '"'; i++ {
        if self.lex.src[i] == '\\' {
            i++
        }
    }

    /* check for EOF */
    if i == len(self.lex.src) {
        panic(self.err(i, "unexpected EOF when scanning strings"))
    }

    /* unquote the string */
    if v, e = strconv.Unquote(string(self.lex.src[p:i + 1])); e != nil {
        panic(self.err(p, "invalid string: " + e.Error()))
    }

    /* add the argument to buffer */
    *argv = append(*argv, ParsedCommandArg { Value: v, IsString: true })
    return self.delim(i + 1)
}

func (self *Parser) directives(line string) {
    self.lex.row++
    self.lex.init(line)

    /* parse the first token */
    tk := self.lex.next()
    tt := tk.tag

    /* check for EOF */
    if tt == _T_end {
        return
    }

    /* must be a directive */
    if tt != _T_punc || tk.punc() != _P_hash {
        panic(self.err(tk.pos, "'#' expected"))
    }

    /* parse the line number */
    tk = self.lex.next()
    tt = tk.tag

    /* must be a line number, if it is, set the row number, and ignore the rest of the line */
    if tt != _T_int {
        panic(self.err(tk.pos, "line number expected"))
    } else {
        self.lex.row = int(tk.u64) - 1
    }
}

func (self *Parser) expressions(argv *[]ParsedCommandArg, p int) int {
    var i int
    var n int
    var s int

    /* scan until the first standalone ',' or EOF */
    loop: for i = p; i < len(self.lex.src); i++ {
        switch self.lex.src[i] {
            case ','           : if s == 0 { if n == 0 { break loop } }
            case ']', '}', '>' : if s == 0 { if n == 0 { break loop } else { n-- } }
            case '[', '{', '<' : if s == 0 { n++ }
            case '\\'          : if s != 0 { i++ }
            case '\''          : if s != 2 { s ^= 1 }
            case '"'           : if s != 1 { s ^= 2 }
        }
    }

    /* check for EOF in strings */
    if s != 0 {
        panic(self.err(i, "unexpected EOF when scanning strings"))
    }

    /* check for bracket matching */
    if n != 0 {
        panic(self.err(i, "unbalanced '{' or '[' or '<'"))
    }

    /* add the argument to buffer */
    *argv = append(*argv, ParsedCommandArg { Value: string(self.lex.src[p:i]) })
    return self.delim(i)
}

// Feed feeds the parser with one more line, and the parser
// parses it into a ParsedLine.
//
// NOTE: Feed does not handle empty lines or multiple lines,
//       it panics when this happens. Use Parse to parse multiple
//       lines of assembly source.
//
func (self *Parser) Feed(src string) (ret *ParsedLine, err error) {
    var ok bool
    var ss string
    var vv interface{}

    /* check for multiple lines */
    if strings.ContainsRune(src, '\n') {
        return nil, errors.New("passing multiple lines to Feed()")
    }

    /* check for blank lines */
    if ss = strings.TrimSpace(src); ss == "" || ss[0] == '#' || strings.HasPrefix(ss, "//") {
        return nil, errors.New("blank line or line with only comments or line-marks")
    }

    /* setup error handler */
    defer func() {
        if vv = recover(); vv != nil {
            if err, ok = vv.(*SyntaxError); !ok {
                panic(vv)
            }
        }
    }()

    /* call the actual parser */
    ret = self.feed(src)
    return
}

// Parse parses the entire assembly source (possibly multiple lines) into
// a sequence of *ParsedLine.
func (self *Parser) Parse(src string) (ret []*ParsedLine, err error) {
    var ok bool
    var ss string
    var vv interface{}

    /* setup error handler */
    defer func() {
        if vv = recover(); vv != nil {
            if err, ok = vv.(*SyntaxError); !ok {
                panic(vv)
            }
        }
    }()

    /* feed every line */
    for _, line := range strings.Split(src, "\n") {
        if ss = strings.TrimSpace(line); ss == "" || strings.HasPrefix(ss, "//") {
            self.lex.row++
        } else if ss[0] == '#' {
            self.directives(line)
        } else {
            ret = append(ret, self.feed(line))
        }
    }

    /* all done */
    err = nil
    return
}

// Directive handles the directive.
func (self *Parser) Directive(line string) (err error) {
    var ok bool
    var ss string
    var vv interface{}

    /* check for directives */
    if ss = strings.TrimSpace(line); ss == "" || ss[0] != '#' {
        return errors.New("not a directive")
    }

    /* setup error handler */
    defer func() {
        if vv = recover(); vv != nil {
            if err, ok = vv.(*SyntaxError); !ok {
                panic(vv)
            }
        }
    }()

    /* call the directive parser */
    self.directives(line)
    return
}

type _TermRepo struct {
    terms map[string]expr.Term
}

func (self *_TermRepo) Get(name string) (expr.Term, error) {
    if ret, ok := self.terms[name]; ok {
        return ret, nil
    } else {
        return nil, errors.New("undefined name: " + name)
    }
}

func (self *_TermRepo) label(name string) (*Label, error) {
    var ok bool
    var lb *Label
    var tr expr.Term

    /* check for existing terms */
    if tr, ok = self.terms[name]; ok {
        if lb, ok = tr.(*Label); ok {
            return lb, nil
        } else {
            return nil, errors.New("name is not a label: " + name)
        }
    }

    /* create a new one as needed */
    lb = new(Label)
    lb.Name = name

    /* create the map if needed */
    if self.terms == nil {
        self.terms = make(map[string]expr.Term, 1)
    }

    /* register the label */
    self.terms[name] = lb
    return lb, nil
}

func (self *_TermRepo) define(name string, term expr.Term) {
    var ok bool
    var tr expr.Term

    /* create the map if needed */
    if self.terms == nil {
        self.terms = make(map[string]expr.Term, 1)
    }

    /* check for existing terms */
    if tr, ok = self.terms[name]; !ok {
        self.terms[name] = term
    } else if _, ok = tr.(*Label); !ok {
        self.terms[name] = term
    } else {
        panic("conflicting term types: " + name)
    }
}

// _Command describes an assembler command.
//
// The _Command.args describes both the arity and argument type with characters,
// the length is the number of arguments, the character itself represents the
// argument type.
//
// Possible values are:
//
//      s   This argument should be a string
//      e   This argument should be an expression
//      ?   The next argument is optional, and must be the last argument.
//
type _Command struct {
    args    string
    handler func(*Assembler, *Program, []ParsedCommandArg) error
}

// Options controls the behavior of Assembler.
type Options struct {
    // InstructionAliasing specifies whether to enable instruction aliasing.
    // Set to true enables instruction aliasing, and the Assembler will try harder to find instructions.
    InstructionAliasing bool

    // IgnoreUnknownDirectives specifies whether to report errors when encountered unknown directives.
    // Set to true ignores all unknwon directives silently, useful for parsing generated assembly.
    IgnoreUnknownDirectives bool
}

// Assembler assembles the entire assembly program and generates the corresponding
// machine code representations.
type Assembler struct {
    cc   int
    ps   Parser
    pc   uintptr
    buf  []byte
    main string
    opts Options
    repo _TermRepo
    expr expr.Parser
    line *ParsedLine
}

var asmCommands = map[string]_Command {
    "org"     : { "e"   , (*Assembler).assembleCommandOrg     },
    "set"     : { "ee"  , (*Assembler).assembleCommandSet     },
    "byte"    : { "e"   , (*Assembler).assembleCommandByte    },
    "word"    : { "e"   , (*Assembler).assembleCommandWord    },
    "long"    : { "e"   , (*Assembler).assembleCommandLong    },
    "quad"    : { "e"   , (*Assembler).assembleCommandQuad    },
    "fill"    : { "e?e" , (*Assembler).assembleCommandFill    },
    "space"   : { "e?e" , (*Assembler).assembleCommandFill    },
    "align"   : { "e?e" , (*Assembler).assembleCommandAlign   },
    "entry"   : { "e"   , (*Assembler).assembleCommandEntry   },
    "ascii"   : { "s"   , (*Assembler).assembleCommandAscii   },
    "asciz"   : { "s"   , (*Assembler).assembleCommandAsciz   },
    "p2align" : { "e?e" , (*Assembler).assembleCommandP2Align },
}

func (self *Assembler) err(msg string) *SyntaxError {
    return &SyntaxError {
        Pos    : -1,
        Row    : self.line.Row,
        Src    : self.line.Src,
        Reason : msg,
    }
}

func (self *Assembler) eval(expr string) (int64, error) {
    if exp, err := self.expr.SetSource(expr).Parse(nil); err != nil {
        return 0, err
    } else {
        return exp.Evaluate()
    }
}

func (self *Assembler) checkArgs(i int, n int, v *ParsedCommand, isString bool) error {
    if i >= len(v.Args) {
        return self.err(fmt.Sprintf("command %s takes exact %d arguments", strconv.Quote(v.Cmd), n))
    } else if isString && !v.Args[i].IsString {
        return self.err(fmt.Sprintf("argument %d of command %s must be a string", i + 1, strconv.Quote(v.Cmd)))
    } else if !isString && v.Args[i].IsString {
        return self.err(fmt.Sprintf("argument %d of command %s must be an expression", i + 1, strconv.Quote(v.Cmd)))
    } else {
        return nil
    }
}

func (self *Assembler) assembleLabel(p *Program, lb *ParsedLabel) error {
    if v, err := self.repo.label(lb.Name); err != nil {
        return err
    } else {
        p.Link(v)
        return nil
    }
}

func (self *Assembler) assembleInstr(p *Program, line *ParsedInstruction) (err error) {
    var ok  bool
    var pfx []byte
    var ops []interface{}
    var enc _InstructionEncoder

    /* convert to lower-case */
    opts := self.opts
    name := strings.ToLower(line.Mnemonic)

    /* fix register-addressing branches if needed */
    if opts.InstructionAliasing && len(line.Operands) == 1 {
        switch {
            case name == "retq"                                    : name = "ret"
            case name == "movabsq"                                 : name = "movq"
            case name == "jmp"   && line.Operands[0].Op != OpLabel : name = "jmpq"
            case name == "jmpq"  && line.Operands[0].Op == OpLabel : name = "jmp"
            case name == "call"  && line.Operands[0].Op != OpLabel : name = "callq"
            case name == "callq" && line.Operands[0].Op == OpLabel : name = "call"
        }
    }

    /* lookup from the alias table if needed */
    if opts.InstructionAliasing {
        enc, ok = _InstructionAliases[name]
    }

    /* lookup from the instruction table */
    if !ok {
        enc, ok = Instructions[name]
    }

    /* remove size suffix if possible */
    if !ok && opts.InstructionAliasing {
        switch i := len(name) - 1; name[i] {
            case 'b', 'w', 'l', 'q': {
                enc, ok = Instructions[name[:i]]
            }
        }
    }

    /* check for instruction name */
    if !ok {
        return self.err("no such instruction: " + strconv.Quote(name))
    }

    /* allocate memory for prefix if any */
    if len(line.Prefixes) != 0 {
        pfx = make([]byte, len(line.Prefixes))
    }

    /* convert the prefixes */
    for i, v := range line.Prefixes {
        switch v {
            case PrefixLock      : pfx[i] = _P_lock
            case PrefixSegmentCS : pfx[i] = _P_cs
            case PrefixSegmentDS : pfx[i] = _P_ds
            case PrefixSegmentES : pfx[i] = _P_es
            case PrefixSegmentFS : pfx[i] = _P_fs
            case PrefixSegmentGS : pfx[i] = _P_gs
            case PrefixSegmentSS : pfx[i] = _P_ss
            default              : panic("unreachable: invalid segment prefix")
        }
    }

    /* convert the operands */
    for _, op := range line.Operands {
        switch op.Op {
            case OpImm   : ops = append(ops, op.Imm)
            case OpReg   : ops = append(ops, op.Reg)
            case OpMem   : self.assembleInstrMem(&ops, op.Memory)  
            case OpLabel : self.assembleInstrLabel(&ops, op.Label) 
            default      : panic("parser yields an invalid operand kind")
        }
    }

    /* catch any exceptions in the encoder */
    defer func() {
        if v := recover(); v != nil {
            err = self.err(fmt.Sprint(v))
        }
    }()

    /* encode the instruction */
    enc(p, ops...).prefix = pfx
    return nil
}

func (self *Assembler) assembleInstrMem(ops *[]interface{}, addr MemoryAddress) {
    mem := new(MemoryOperand)
    *ops = append(*ops, mem)

    /* check for RIP-relative addressing */
    if addr.Base != rip {
        mem.Addr.Type = Memory
        mem.Addr.Memory = addr
    } else {
        mem.Addr.Type = Offset
        mem.Addr.Offset = RelativeOffset(addr.Displacement)
    }
}

func (self *Assembler) assembleInstrLabel(ops *[]interface{}, label ParsedLabel) {
    vk := label.Kind
    tr, err := self.repo.label(label.Name)

    /* check for errors */
    if err != nil {
        panic(err)
    }

    /* check for branch target */
    if vk == BranchTarget {
        *ops = append(*ops, tr)
        return
    }

    /* add to ops */
    *ops = append(*ops, &MemoryOperand {
        Addr: Addressable {
            Type      : Reference,
            Reference : tr,
        },
    })
}

func (self *Assembler) assembleCommand(p *Program, line *ParsedCommand) error {
    var iv int
    var cc rune
    var ok bool
    var va bool
    var fn _Command

    /* find the command */
    if fn, ok = asmCommands[line.Cmd]; !ok {
        if self.opts.IgnoreUnknownDirectives {
            return nil
        } else {
            return self.err("no such command: " + strconv.Quote(line.Cmd))
        }
    }

    /* expected & real argument count */
    argx := len(fn.args)
    argc := len(line.Args)

    /* check the arguments */
    loop: for iv, cc = range fn.args {
        switch cc {
            case '?' : va = true; break loop
            case 's' : if err := self.checkArgs(iv, argx, line, true)  ; err != nil { return err }
            case 'e' : if err := self.checkArgs(iv, argx, line, false) ; err != nil { return err }
            default  : panic("invalid argument descriptor: " + strconv.Quote(fn.args))
        }
    }

    /* simple case: non-variadic command */
    if !va {
        if argc == argx {
            return fn.handler(self, p, line.Args)
        } else {
            return self.err(fmt.Sprintf("command %s takes exact %d arguments", strconv.Quote(line.Cmd), argx))
        }
    }

    /* check for the descriptor */
    if iv != argx - 2 {
        panic("invalid argument descriptor: " + strconv.Quote(fn.args))
    }

    /* variadic command and the final optional argument is set */
    if argc == argx - 1 {
        switch fn.args[argx - 1] {
            case 's' : if err := self.checkArgs(iv, -1, line, true)  ; err != nil { return err }
            case 'e' : if err := self.checkArgs(iv, -1, line, false) ; err != nil { return err }
            default  : panic("invalid argument descriptor: " + strconv.Quote(fn.args))
        }
    }

    /* check argument count */
    if argc == argx - 1 || argc == argx - 2 {
        return fn.handler(self, p, line.Args)
    } else {
        return self.err(fmt.Sprintf("command %s takes %d or %d arguments", strconv.Quote(line.Cmd), argx - 2, argx - 1))
    }
}

func (self *Assembler) assembleCommandInt(p *Program, argv []ParsedCommandArg, addfn func(*Program, *expr.Expr) *Instruction) error {
    var err error
    var val *expr.Expr

    /* parse the expression */
    if val, err = self.expr.SetSource(argv[0].Value).Parse(&self.repo); err != nil {
        return err
    }

    /* add to the program */
    addfn(p, val)
    return nil
}

func (self *Assembler) assembleCommandOrg(_ *Program, argv []ParsedCommandArg) error {
    var err error
    var val int64

    /* evaluate the expression */
    if val, err = self.eval(argv[0].Value); err != nil {
        return err
    }

    /* check for origin */
    if val < 0 {
        return self.err(fmt.Sprintf("negative origin: %d", val))
    }

    /* ".org" must be the first command if any */
    if self.cc != 1 {
        return self.err(".org must be the first command if present")
    }

    /* set the initial program counter */
    self.pc = uintptr(val)
    return nil
}

func (self *Assembler) assembleCommandSet(_ *Program, argv []ParsedCommandArg) error {
    var err error
    var val *expr.Expr

    /* parse the expression */
    if val, err = self.expr.SetSource(argv[1].Value).Parse(&self.repo); err != nil {
        return err
    }

    /* define the new identifier */
    self.repo.define(argv[0].Value, val)
    return nil
}

func (self *Assembler) assembleCommandByte(p *Program, argv []ParsedCommandArg) error {
    return self.assembleCommandInt(p, argv, (*Program).Byte)
}

func (self *Assembler) assembleCommandWord(p *Program, argv []ParsedCommandArg) error {
    return self.assembleCommandInt(p, argv, (*Program).Word)
}

func (self *Assembler) assembleCommandLong(p *Program, argv []ParsedCommandArg) error {
    return self.assembleCommandInt(p, argv, (*Program).Long)
}

func (self *Assembler) assembleCommandQuad(p *Program, argv []ParsedCommandArg) error {
    return self.assembleCommandInt(p, argv, (*Program).Quad)
}

func (self *Assembler) assembleCommandFill(p *Program, argv []ParsedCommandArg) error {
    var fv byte
    var nb int64
    var ex error

    /* evaluate the size */
    if nb, ex = self.eval(argv[0].Value); ex != nil {
        return ex
    }

    /* check for filling size */
    if nb < 0 {
        return self.err(fmt.Sprintf("negative filling size: %d", nb))
    }

    /* check for optional filling value */
    if len(argv) == 2 {
        if val, err := self.eval(argv[1].Value); err != nil {
            return err
        } else if val < math.MinInt8 || val > math.MaxUint8 {
            return self.err(fmt.Sprintf("value %d cannot be represented with a byte", val))
        } else {
            fv = byte(val)
        }
    }

    /* fill with specified byte */
    p.Data(bytes.Repeat([]byte { fv }, int(nb)))
    return nil
}

func (self *Assembler) assembleCommandAlign(p *Program, argv []ParsedCommandArg) error {
    var nb int64
    var ex error
    var fv *expr.Expr

    /* evaluate the size */
    if nb, ex = self.eval(argv[0].Value); ex != nil {
        return ex
    }

    /* check for alignment value */
    if nb <= 0 {
        return self.err(fmt.Sprintf("zero or negative alignment: %d", nb))
    }

    /* alignment must be a power of 2 */
    if (nb & (nb - 1)) != 0 {
        return self.err(fmt.Sprintf("alignment must be a power of 2: %d", nb))
    }

    /* check for optional filling value */
    if len(argv) == 2 {
        if v, err := self.expr.SetSource(argv[1].Value).Parse(&self.repo); err == nil {
            fv = v
        } else {
            return err
        }
    }

    /* fill with specified byte, default to 0 if not specified */
    p.Align(uint64(nb), fv)
    return nil
}

func (self *Assembler) assembleCommandEntry(_ *Program, argv []ParsedCommandArg) error {
    name := argv[0].Value
    rbuf := []rune(name)

    /* check all the characters */
    for i, cc := range rbuf {
        if !isident0(cc) && (i == 0 || !isident(cc)) {
            return self.err("entry point must be a label name")
        }
    }

    /* set the main entry point */
    self.main = name
    return nil
}

func (self *Assembler) assembleCommandAscii(p *Program, argv []ParsedCommandArg) error {
    p.Data([]byte(argv[0].Value))
    return nil
}

func (self *Assembler) assembleCommandAsciz(p *Program, argv []ParsedCommandArg) error {
    p.Data(append([]byte(argv[0].Value), 0))
    return nil
}

func (self *Assembler) assembleCommandP2Align(p *Program, argv []ParsedCommandArg) error {
    var nb int64
    var ex error
    var fv *expr.Expr

    /* evaluate the size */
    if nb, ex = self.eval(argv[0].Value); ex != nil {
        return ex
    }

    /* check for alignment value */
    if nb <= 0 {
        return self.err(fmt.Sprintf("zero or negative alignment: %d", nb))
    }

    /* check for optional filling value */
    if len(argv) == 2 {
        if v, err := self.expr.SetSource(argv[1].Value).Parse(&self.repo); err == nil {
            fv = v
        } else {
            return err
        }
    }

    /* fill with specified byte, default to 0 if not specified */
    p.Align(1 << nb, fv)
    return nil
}

// Base returns the origin.
func (self *Assembler) Base() uintptr {
    return self.pc
}

// Code returns the assembled machine code.
func (self *Assembler) Code() []byte {
    return self.buf
}

// Entry returns the address of the specified entry point, or the origin if not specified.
func (self *Assembler) Entry() uintptr {
    if self.main == "" {
        return self.pc
    } else if tr, err := self.repo.Get(self.main); err != nil {
        panic(err)
    } else if val, err := tr.Evaluate(); err != nil {
        panic(err)
    } else {
        return uintptr(val)
    }
}

// Options returns the internal options reference, changing it WILL affect this Assembler instance.
func (self *Assembler) Options() *Options {
    return &self.opts
}

// WithBase resets the origin to pc.
func (self *Assembler) WithBase(pc uintptr) *Assembler {
    self.pc = pc
    return self
}

// Assemble assembles the assembly source and save the machine code to internal buffer.
func (self *Assembler) Assemble(src string) error {
    var err error
    var buf []*ParsedLine

    /* parse the source */
    if buf, err = self.ps.Parse(src); err != nil {
        return err
    }

    /* create a new program */
    p := DefaultArch.CreateProgram()
    defer p.Free()

    /* process every line */
    for _, self.line = range buf {
        switch self.cc++; self.line.Kind {
            case LineLabel   : if err = self.assembleLabel   (p, &self.line.Label)       ; err != nil { return err }
            case LineInstr   : if err = self.assembleInstr   (p, &self.line.Instruction) ; err != nil { return err }
            case LineCommand : if err = self.assembleCommand (p, &self.line.Command)     ; err != nil { return err }
            default          : panic("parser yields an invalid line kind")
        }
    }

    /* assemble the program */
    self.buf = p.Assemble(self.pc)
    return nil
}
