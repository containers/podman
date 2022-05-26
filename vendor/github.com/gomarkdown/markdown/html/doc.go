/*
Package html implements HTML renderer of parsed markdown document.

Configuring and customizing a renderer

A renderer can be configured with multiple options:

	import "github.com/gomarkdown/markdown/html"

	flags := html.CommonFlags | html.CompletePage | html.HrefTargetBlank
	opts := html.RendererOptions{
		Title: "A custom title",
		Flags: flags,
	}
	renderer := html.NewRenderer(opts)

You can also re-use most of the logic and customize rendering of selected nodes
by providing node render hook.
This is most useful for rendering nodes that allow for design choices, like
links or code blocks.

	import (
		"github.com/gomarkdown/markdown/html"
		"github.com/gomarkdown/markdown/ast"
	)

	// a very dummy render hook that will output "code_replacements" instead of
	// <code>${content}</code> emitted by html.Renderer
	func renderHookCodeBlock(w io.Writer, node *ast.Node, entering bool) (ast.WalkStatus, bool) {
		_, ok := node.Data.(*ast.CodeBlockData)
		if !ok {
			return ast.GoToNext, false
		}
		io.WriteString(w, "code_replacement")
		return ast.GoToNext, true
	}

	opts := html.RendererOptions{
		RenderNodeHook: renderHookCodeBlock,
	}
	renderer := html.NewRenderer(opts)
*/
package html
