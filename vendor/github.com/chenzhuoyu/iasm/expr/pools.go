package expr

import (
    `sync`
)

var (
    expressionPool sync.Pool
)

func newExpression() *Expr {
    if v := expressionPool.Get(); v == nil {
        return new(Expr)
    } else {
        return resetExpression(v.(*Expr))
    }
}

func freeExpression(p *Expr) {
    expressionPool.Put(p)
}

func resetExpression(p *Expr) *Expr {
    *p = Expr{}
    return p
}
