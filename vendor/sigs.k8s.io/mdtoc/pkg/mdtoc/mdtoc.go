/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mdtoc

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"regexp"
	"strings"

	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/mmarkdown/mmark/mparser"
)

const (
	// StartTOC is the opening tag for the table of contents
	StartTOC = "<!-- toc -->"
	// EndTOC is the tag that marks the end of the TOC
	EndTOC = "<!-- /toc -->"
	//
	MaxHeaderDepth = 6
)

var (
	startTOCRegex = regexp.MustCompile("(?i)" + StartTOC)
	endTOCRegex   = regexp.MustCompile("(?i)" + EndTOC)
)

// Options set for the toc generator
type Options struct {
	Dryrun     bool
	SkipPrefix bool
	MaxDepth   int
}

// parse parses a raw markdown document to an AST.
func parse(b []byte) ast.Node {
	p := parser.NewWithExtensions(parser.CommonExtensions)
	p.Opts = parser.Options{
		// mparser is required for parsing the --- title blocks
		ParserHook: mparser.Hook,
	}
	return p.Parse(b)
}

// GenerateTOC parses a document and returns its TOC
func GenerateTOC(doc []byte, opts Options) (string, error) {
	anchors := make(anchorGen)

	md := parse(doc)

	baseLvl := headingBase(md)
	toc := &bytes.Buffer{}
	htmlRenderer := html.NewRenderer(html.RendererOptions{})
	walkHeadings(md, func(heading *ast.Heading) {
		if heading.Level > opts.MaxDepth {
			return
		}
		anchor := anchors.mkAnchor(asText(heading))
		content := headingBody(htmlRenderer, heading)
		fmt.Fprintf(toc, "%s- [%s](#%s)\n", strings.Repeat("  ", heading.Level-baseLvl), content, anchor)
	})

	return toc.String(), nil
}

type headingFn func(heading *ast.Heading)

// walkHeadings runs the heading function on each heading in the parsed markdown document.
func walkHeadings(doc ast.Node, headingFn headingFn) {
	ast.WalkFunc(doc, func(node ast.Node, entering bool) ast.WalkStatus {
		if !entering {
			return ast.GoToNext // Don't care about closing the heading section.
		}

		heading, ok := node.(*ast.Heading)
		if !ok {
			return ast.GoToNext // Ignore non-heading nodes.
		}

		if heading.IsTitleblock {
			return ast.GoToNext // Ignore title blocks (the --- section)
		}

		headingFn(heading)

		return ast.GoToNext
	})
}

// anchorGen is used to generate heading anchor IDs, using the github-flavored markdown syntax.
type anchorGen map[string]int

func (a anchorGen) mkAnchor(text string) string {
	text = strings.ToLower(text)
	text = punctuation.ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, " ", "-")
	idx := a[text]
	a[text] = idx + 1
	if idx > 0 {
		return fmt.Sprintf("%s-%d", text, idx)
	}
	return text
}

// Locate the case-insensitive TOC tags.
func findTOCTags(raw []byte) (start, end int) {
	if ind := startTOCRegex.FindIndex(raw); len(ind) > 0 {
		start = ind[0]
	} else {
		start = -1
	}
	if ind := endTOCRegex.FindIndex(raw); len(ind) > 0 {
		end = ind[0]
	} else {
		end = -1
	}
	return
}

func asText(node ast.Node) (text string) {
	ast.WalkFunc(node, func(node ast.Node, entering bool) ast.WalkStatus {
		if !entering {
			return ast.GoToNext // Don't care about closing the heading section.
		}

		switch node.(type) {
		case *ast.Text, *ast.Code:
			text += string(node.AsLeaf().Literal)
		}

		return ast.GoToNext
	})
	return text
}

// Renders the heading body as HTML
func headingBody(renderer *html.Renderer, heading *ast.Heading) string {
	var buf bytes.Buffer
	for _, child := range heading.Children {
		ast.WalkFunc(child, func(node ast.Node, entering bool) ast.WalkStatus {
			return renderer.RenderNode(&buf, node, entering)
		})
	}
	return strings.TrimSpace(buf.String())
}

// headingBase finds the minimum heading level. This is useful for normalizing indentation, such as
// when a top-level heading is skipped in the prefix.
func headingBase(doc ast.Node) int {
	baseLvl := math.MaxInt32
	walkHeadings(doc, func(heading *ast.Heading) {
		if baseLvl > heading.Level {
			baseLvl = heading.Level
		}
	})

	return baseLvl
}

// Match punctuation that is filtered out from anchor IDs.
var punctuation = regexp.MustCompile(`[^\w\- ]`)

// WriteTOC writes the TOC generator on file with options.
// Returns the generated toc, and any error.
func WriteTOC(file string, opts Options) error {
	raw, err := ioutil.ReadFile(file)
	if err != nil {
		return fmt.Errorf("unable to read %s: %v", file, err)
	}

	start, end := findTOCTags(raw)

	if start == -1 {
		return fmt.Errorf("missing opening TOC tag")
	}
	if end == -1 {
		return fmt.Errorf("missing closing TOC tag")
	}
	if end < start {
		return fmt.Errorf("TOC closing tag before start tag")
	}

	var doc []byte
	doc = raw
	// skipPrefix is only used when toc tags are present.
	if opts.SkipPrefix && start != -1 && end != -1 {
		doc = raw[end:]
	}
	toc, err := GenerateTOC(doc, opts)
	if err != nil {
		return fmt.Errorf("failed to generate toc: %v", err)
	}

	realStart := start + len(StartTOC)
	oldTOC := string(raw[realStart:end])
	if strings.TrimSpace(oldTOC) == strings.TrimSpace(toc) {
		// No changes required.
		return nil
	} else if opts.Dryrun {
		return fmt.Errorf("changes found:\n%s", toc)
	}

	err = atomicWrite(file,
		string(raw[:realStart])+"\n",
		toc,
		string(raw[end:]),
	)
	return err
}

// GetTOC generates the TOC from a file with options.
// Returns the generated toc, and any error.
func GetTOC(file string, opts Options) (string, error) {
	doc, err := ioutil.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("unable to read %s: %v", file, err)
	}

	start, end := findTOCTags(doc)
	startPos := 0

	// skipPrefix is only used when toc tags are present.
	if opts.SkipPrefix && start != -1 && end != -1 {
		startPos = end
	}
	toc, err := GenerateTOC(doc[startPos:], opts)
	if err != nil {
		return toc, fmt.Errorf("failed to generate toc: %v", err)
	}

	return toc, err
}

// atomicWrite writes the chunks sequentially to the filePath.
// A temporary file is used so no changes are made to the original in the case of an error.
func atomicWrite(filePath string, chunks ...string) error {
	tmpPath := filePath + "_tmp"
	tmp, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("unable to open tepmorary file %s: %v", tmpPath, err)
	}

	// Cleanup
	defer func() {
		tmp.Close()
		os.Remove(tmpPath)
	}()

	for _, chunk := range chunks {
		if _, err := tmp.WriteString(chunk); err != nil {
			return err
		}
	}

	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filePath)
}
