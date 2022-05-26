package mast

import (
	"bytes"
	"sort"

	"github.com/gomarkdown/markdown/ast"
)

// some extra functions for manipulation the AST

// MoveChilderen moves the children from a to b *and* make the parent of each point to b.
// Any children of b are obliterated.
func MoveChildren(a, b ast.Node) {
	a.SetChildren(b.GetChildren())
	b.SetChildren(nil)

	for _, child := range a.GetChildren() {
		child.SetParent(a)
	}
}

// Some attribute helper functions.

// AttributeFromNode returns the attribute from the node, if it was there was one.
func AttributeFromNode(node ast.Node) *ast.Attribute {
	if c := node.AsContainer(); c != nil && c.Attribute != nil {
		return c.Attribute
	}
	if l := node.AsLeaf(); l != nil && l.Attribute != nil {
		return l.Attribute
	}
	return nil
}

// AttributeInit will initialize an *Attribute on node if there wasn't one.
func AttributeInit(node ast.Node) {
	if l := node.AsLeaf(); l != nil && l.Attribute == nil {
		l.Attribute = &ast.Attribute{Attrs: make(map[string][]byte)}
		return
	}
	if c := node.AsContainer(); c != nil && c.Attribute == nil {
		c.Attribute = &ast.Attribute{Attrs: make(map[string][]byte)}
		return
	}
}

// DeleteAttribute delete the attribute under key from a.
func DeleteAttribute(node ast.Node, key string) {
	a := AttributeFromNode(node)
	if a == nil {
		return
	}

	switch key {
	case "id":
		a.ID = nil
	case "class":
		// TODO
	default:
		delete(a.Attrs, key)
	}
}

// SetAttribute sets the attribute under key to value.
func SetAttribute(node ast.Node, key string, value []byte) {
	a := AttributeFromNode(node)
	if a == nil {
		return
	}
	switch key {
	case "id":
		a.ID = value
	case "class":
		// TODO
	default:
		a.Attrs[key] = value
	}
}

// Attribute returns the attribute value under key. Use AttributeClass to retrieve
// a class.
func Attribute(node ast.Node, key string) []byte {
	a := AttributeFromNode(node)
	if a == nil {
		return nil
	}
	switch key {
	case "id":
		return a.ID
	case "class":
		// use AttributeClass.
	}

	return a.Attrs[key]
}

func AttributeBytes(attr *ast.Attribute) []byte {
	ret := &bytes.Buffer{}
	ret.WriteByte('{')
	if len(attr.ID) != 0 {
		ret.WriteByte('#')
		ret.Write(attr.ID)
	}
	for _, c := range attr.Classes {
		if ret.Len() > 1 {
			ret.WriteByte(' ')
		}
		ret.WriteByte('.')
		ret.Write(c)
	}

	keys := []string{}
	for k := range attr.Attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if ret.Len() > 1 {
			ret.WriteByte(' ')
		}
		ret.WriteString(k)
		ret.WriteString(`="`)
		ret.Write(attr.Attrs[k])
		ret.WriteByte('"')
	}
	ret.WriteByte('}')
	return ret.Bytes()
}

// AttributeClass returns true is class key is set.
func AttributeClass(node ast.Node, key string) bool {
	a := AttributeFromNode(node)
	if a == nil {
		return false
	}
	for _, c := range a.Classes {
		if string(c) == key {
			return true
		}
	}
	return false
}

// AttributeFilter runs the attribute on node through filter and only allows elements for which filter returns true.
func AttributeFilter(node ast.Node, filter func(key string) bool) {
	a := AttributeFromNode(node)
	if a == nil {
		return
	}
	if !filter("id") {
		a.ID = nil
	}
	if !filter("class") {
		a.Classes = nil
	}
	for k, _ := range a.Attrs {
		if !filter(k) {
			delete(a.Attrs, k)
		}
	}
}

// FilterFunc checks if s is an allowed key in an attribute.
// If s is:
// "id" the ID should be checked
// "class" the classes should be allowed or disallowed
// any other string means checking the individual attributes.
// it returns true for elements that are allows, false otherwise.
type FilterFunc func(s string) bool
