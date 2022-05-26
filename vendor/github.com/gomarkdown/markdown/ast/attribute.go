package ast

// An attribute can be attached to block elements. They are specified as
// {#id .classs key="value"} where quotes for values are mandatory, multiple
// key/value pairs are separated by whitespace.
type Attribute struct {
	ID      []byte
	Classes [][]byte
	Attrs   map[string][]byte
}
