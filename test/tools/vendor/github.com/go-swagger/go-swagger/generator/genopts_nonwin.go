//go:build !windows
// +build !windows

package generator

import (
	"log"
	"plugin"
	"text/template"
)

type GenOpts struct {
	GenOptsCommon
	TemplatePlugin string
}

func (g *GenOpts) setTemplates() error {
	if g.TemplatePlugin != "" {
		if err := g.templates.LoadPlugin(g.TemplatePlugin); err != nil {
			return err
		}
	}

	return g.GenOptsCommon.setTemplates()
}

// LoadPlugin will load the named plugin and inject its functions into the funcMap
//
// The plugin must implement a function matching the signature:
// `func AddFuncs(f template.FuncMap)`
// which can add any number of functions to the template repository funcMap.
// Any existing sprig or go-swagger templates with the same name will be overridden.
func (t *Repository) LoadPlugin(pluginPath string) error {
	log.Printf("Attempting to load template plugin: %s", pluginPath)

	p, err := plugin.Open(pluginPath)
	if err != nil {
		return err
	}

	f, err := p.Lookup("AddFuncs")
	if err != nil {
		return err
	}

	f.(func(template.FuncMap))(t.funcs)
	return nil
}
