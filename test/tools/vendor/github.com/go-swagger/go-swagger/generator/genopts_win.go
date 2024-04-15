//go:build windows
// +build windows

package generator

type GenOpts struct {
	GenOptsCommon
}

func (g *GenOpts) setTemplates() error {
	return g.GenOptsCommon.setTemplates()
}
