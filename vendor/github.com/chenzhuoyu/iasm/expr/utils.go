package expr

var op1ch = [...]bool {
    '+': true,
    '-': true,
    '*': true,
    '/': true,
    '%': true,
    '&': true,
    '|': true,
    '^': true,
    '~': true,
    '(': true,
    ')': true,
}

var op2ch = [...]bool {
    '*': true,
    '<': true,
    '>': true,
}

func neg2(v *Expr, err error) (*Expr, error) {
    if err != nil {
        return nil, err
    } else {
        return v.Neg(), nil
    }
}

func not2(v *Expr, err error) (*Expr, error) {
    if err != nil {
        return nil, err
    } else {
        return v.Not(), nil
    }
}

func isop1ch(ch rune) bool {
    return ch >= 0 && int(ch) < len(op1ch) && op1ch[ch]
}

func isop2ch(ch rune) bool {
    return ch >= 0 && int(ch) < len(op2ch) && op2ch[ch]
}

func isdigit(ch rune) bool {
    return ch >= '0' && ch <= '9'
}

func isident(ch rune) bool {
    return isdigit(ch) || isident0(ch)
}

func isident0(ch rune) bool {
    return (ch == '_') || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func ishexdigit(ch rune) bool {
    return isdigit(ch) || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}
