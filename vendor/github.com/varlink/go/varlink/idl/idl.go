// Package idl provides a varlink interface description parser.
package idl

import (
	"bytes"
	"fmt"
	"regexp"
)

// Valid TypeKind values.
const (
	TypeBool = iota
	TypeInt
	TypeFloat
	TypeString
	TypeArray
	TypeMaybe
	TypeStruct
	TypeEnum
	TypeAlias
)

// TypeKind specifies the type of an Type.
type TypeKind uint

// Type represents a varlink type. Types are method input and output parameters,
// error output parameters, or custom defined types in the interface description.
type Type struct {
	Kind        TypeKind
	ElementType *Type
	Alias       string
	Fields      []TypeField
}

// TypeField is a named member of a TypeStruct.
type TypeField struct {
	Name string
	Type *Type
}

// Alias represents a named Type in the interface description.
type Alias struct {
	Name string
	Doc  string
	Type *Type
}

// Method represents a method defined in the interface description.
type Method struct {
	Name string
	Doc  string
	In   *Type
	Out  *Type
}

// Error represents an error defined in the interface description.
type Error struct {
	Name string
	Type *Type
}

// IDL represents a parsed varlink interface description with types, methods, errors and
// documentation.
type IDL struct {
	Name        string
	Doc         string
	Description string
	Members     []interface{}
	Aliases     map[string]*Alias
	Methods     map[string]*Method
	Errors      map[string]*Error
}

type parser struct {
	input       string
	position    int
	lineStart   int
	lastComment bytes.Buffer
}

func (p *parser) next() int {
	r := -1

	if p.position < len(p.input) {
		r = int(p.input[p.position])
	}

	p.position++
	return r
}

func (p *parser) backup() {
	p.position--
}

func (p *parser) advance() bool {
	for {
		char := p.next()

		if char == '\n' {
			p.lineStart = p.position
			p.lastComment.Reset()

		} else if char == ' ' || char == '\t' {
			// ignore

		} else if char == '#' {
			p.next()
			start := p.position
			for {
				c := p.next()
				if c < 0 || c == '\n' {
					p.backup()
					break
				}
			}
			if p.lastComment.Len() > 0 {
				p.lastComment.WriteByte('\n')
			}
			p.lastComment.WriteString(p.input[start:p.position])
			p.next()

		} else {
			p.backup()
			break
		}
	}

	return p.position < len(p.input)
}

func (p *parser) advanceOnLine() {
	for {
		char := p.next()
		if char != ' ' {
			p.backup()
			return
		}
	}
}

func (p *parser) readKeyword() string {
	start := p.position

	for {
		char := p.next()
		if char < 'a' || char > 'z' {
			p.backup()
			break
		}
	}

	return p.input[start:p.position]
}

func (p *parser) readInterfaceName() string {
	start := p.position
	dnrx := regexp.MustCompile(`^[a-z]+(\.[a-z0-9]+([-][a-z0-9]+)*)+`)
	name := dnrx.FindString(p.input[start:])
	if name != "" {
		if len(name) > 255 {
			return ""
		}
		p.position += len(name)
		return name
	}
	xdnrx := regexp.MustCompile(`^xn--[a-z0-9]+(\.[a-z0-9]+([-][a-z0-9]+)*)+`)
	name = xdnrx.FindString(p.input[start:])
	if name != "" {
		if len(name) > 255 {
			return ""
		}
		p.position += len(name)
		return name
	}
	return ""
}

func (p *parser) readFieldName() string {
	start := p.position

	char := p.next()
	if char < 'a' || char > 'z' {
		p.backup()
		return ""
	}

	for {
		char := p.next()
		if (char < 'A' || char > 'Z') && (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '_' {
			p.backup()
			break
		}
	}

	return p.input[start:p.position]
}

func (p *parser) readTypeName() string {
	start := p.position

	for {
		char := p.next()
		if (char < 'A' || char > 'Z') && (char < 'a' || char > 'z') && (char < '0' || char > '9') {
			p.backup()
			break
		}
	}

	return p.input[start:p.position]
}

func (p *parser) readStructType() *Type {
	if p.next() != '(' {
		p.backup()
		return nil
	}

	t := &Type{Kind: TypeStruct}
	t.Fields = make([]TypeField, 0)

	char := p.next()
	if char != ')' {
		p.backup()

		for {
			field := TypeField{}

			p.advance()
			field.Name = p.readFieldName()
			if field.Name == "" {
				return nil
			}

			p.advance()

			// Enums have no types, they are just a list of names
			if p.next() == ':' {
				if t.Kind == TypeEnum {
					return nil
				}

				p.advance()
				field.Type = p.readType()
				if field.Type == nil {
					return nil
				}

			} else {
				t.Kind = TypeEnum
				p.backup()
			}

			t.Fields = append(t.Fields, field)

			p.advance()
			char = p.next()
			if char != ',' {
				break
			}
		}

		if char != ')' {
			return nil
		}
	}

	return t
}

func (p *parser) readType() *Type {
	var t *Type

	switch p.next() {
	case '?':
		e := p.readType()
		if e == nil {
			return nil
		}
		t = &Type{Kind: TypeMaybe, ElementType: e}

	case '[':
		if p.next() != ']' {
			return nil
		}
		e := p.readType()
		if e == nil {
			return nil
		}
		t = &Type{Kind: TypeArray, ElementType: e}

	default:
		p.backup()
		if keyword := p.readKeyword(); keyword != "" {
			switch keyword {
			case "bool":
				t = &Type{Kind: TypeBool}

			case "int":
				t = &Type{Kind: TypeInt}

			case "float":
				t = &Type{Kind: TypeFloat}

			case "string":
				t = &Type{Kind: TypeString}
			}

		} else if name := p.readTypeName(); name != "" {
			t = &Type{Kind: TypeAlias, Alias: name}

		} else if t = p.readStructType(); t == nil {
			return nil
		}
	}

	return t
}

func (p *parser) readAlias(idl *IDL) (*Alias, error) {
	a := &Alias{}

	p.advance()
	a.Doc = p.lastComment.String()
	a.Name = p.readTypeName()
	if a.Name == "" {
		return nil, fmt.Errorf("missing type name")
	}

	p.advance()
	a.Type = p.readType()
	if a.Type == nil {
		return nil, fmt.Errorf("missing type declaration")
	}

	return a, nil
}

func (p *parser) readMethod(idl *IDL) (*Method, error) {
	m := &Method{}

	p.advance()
	m.Doc = p.lastComment.String()
	m.Name = p.readTypeName()
	if m.Name == "" {
		return nil, fmt.Errorf("missing method type")
	}

	p.advance()
	m.In = p.readType()
	if m.In == nil {
		return nil, fmt.Errorf("missing method input")
	}

	p.advance()
	one := p.next()
	two := p.next()
	if (one != '-') || two != '>' {
		return nil, fmt.Errorf("missing method '->' operator")
	}

	p.advance()
	m.Out = p.readType()
	if m.Out == nil {
		return nil, fmt.Errorf("missing method output")
	}

	return m, nil
}

func (p *parser) readError(idl *IDL) (*Error, error) {
	e := &Error{}

	p.advance()
	e.Name = p.readTypeName()
	if e.Name == "" {
		return nil, fmt.Errorf("missing error name")
	}

	p.advanceOnLine()
	e.Type = p.readType()

	return e, nil
}

func (p *parser) readIDL() (*IDL, error) {
	if keyword := p.readKeyword(); keyword != "interface" {
		return nil, fmt.Errorf("missing interface keyword")
	}

	idl := &IDL{
		Members: make([]interface{}, 0),
		Aliases: make(map[string]*Alias),
		Methods: make(map[string]*Method),
		Errors:  make(map[string]*Error),
	}

	p.advance()
	idl.Doc = p.lastComment.String()
	idl.Name = p.readInterfaceName()
	if idl.Name == "" {
		return nil, fmt.Errorf("interface name")
	}

	for {
		if !p.advance() {
			break
		}

		switch keyword := p.readKeyword(); keyword {
		case "type":
			a, err := p.readAlias(idl)
			if err != nil {
				return nil, err
			}

			idl.Members = append(idl.Members, a)
			idl.Aliases[a.Name] = a

		case "method":
			m, err := p.readMethod(idl)
			if err != nil {
				return nil, err
			}

			idl.Members = append(idl.Members, m)
			if _, ok := idl.Methods[m.Name]; ok {
				return nil, fmt.Errorf("method `%s` already defined", m.Name)
			}
			idl.Methods[m.Name] = m

		case "error":
			e, err := p.readError(idl)
			if err != nil {
				return nil, err
			}

			idl.Members = append(idl.Members, e)
			idl.Errors[e.Name] = e

		default:
			return nil, fmt.Errorf("unknown keyword '%s'", keyword)
		}
	}

	return idl, nil
}

// New parses a varlink interface description.
func New(description string) (*IDL, error) {
	p := &parser{input: description}

	p.advance()
	idl, err := p.readIDL()
	if err != nil {
		return nil, err
	}

	if len(idl.Methods) == 0 {
		return nil, fmt.Errorf("no methods defined")
	}

	idl.Description = description
	return idl, nil
}
