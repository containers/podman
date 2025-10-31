//go:build !windows
// +build !windows

package generate

import (
	"github.com/jessevdk/go-flags"

	"github.com/go-swagger/go-swagger/generator"
)

type sharedOptions struct {
	sharedOptionsCommon
	TemplatePlugin flags.Filename `long:"template-plugin" short:"p" description:"the template plugin to use" group:"shared"`
}

func (s sharedOptions) apply(opts *generator.GenOpts) {
	opts.TemplatePlugin = string(s.TemplatePlugin)
	s.sharedOptionsCommon.apply(opts)
}
