package idl

import (
	"fmt"
	"runtime"
	"testing"
)

/*
func expect(t *testing.T, expected string, returned string) {
	if strings.Compare(returned, expected) != 0 {
		t.Fatalf("Expected(%d): `%s`\nGot(%d): `%s`\n",
			len(expected), expected,
			len(returned), returned)
	}
}
*/

func testParse(t *testing.T, pass bool, description string) {
	_, _, line, _ := runtime.Caller(1)

	t.Run(fmt.Sprintf("Line-%d", line), func(t *testing.T) {
		midl, err := New(description)
		if pass {
			if err != nil {
				t.Fatalf("generateTemplate(`%s`): %v", description, err)
			}
			if len(midl.Name) <= 0 {
				t.Fatalf("generateTemplate(`%s`): returned no pkgname", description)
			}
		}
		if !pass && (err == nil) {
			t.Fatalf("generateTemplate(`%s`): did not fail", description)
		}
	})
}

func TestOneMethod(t *testing.T) {
	testParse(t, true, "interface foo.bar\nmethod Foo()->()")
}

func TestOneMethodNoType(t *testing.T) {
	testParse(t, false, "interface foo.bar\nmethod Foo()->(b:)")
}

func TestDomainNames(t *testing.T) {
	testParse(t, true, "interface org.varlink.service\nmethod F()->()")
	testParse(t, true, "interface com.example.0example\nmethod F()->()")
	testParse(t, true, "interface com.example.example-dash\nmethod F()->()")
	testParse(t, true, "interface xn--lgbbat1ad8j.example.algeria\nmethod F()->()")
	testParse(t, false, "interface com.-example.leadinghyphen\nmethod F()->()")
	testParse(t, false, "interface com.example-.danglinghyphen-\nmethod F()->()")
	testParse(t, false, "interface Com.example.uppercase-toplevel\nmethod F()->()")
	testParse(t, false, "interface Co9.example.number-toplevel\nmethod F()->()")
	testParse(t, false, "interface 1om.example.number-toplevel\nmethod F()->()")
	testParse(t, false, "interface com.Example\nmethod F()->()")
	var name string
	for i := 0; i < 255; i++ {
		name += "a"
	}
	testParse(t, false, "interface com.example.toolong"+name+"\nmethod F()->()")
	testParse(t, false, "interface xn--example.toolong"+name+"\nmethod F()->()")
}

func TestNoMethod(t *testing.T) {
	testParse(t, false, `
interface org.varlink.service
  type Interface (name: string, types: []Type, methods: []Method)
  type Property (key: string, value: string)
`)
}

func TestTypeNoArgs(t *testing.T) {
	testParse(t, true, "interface foo.bar\n type I ()\nmethod F()->()")
}

func TestTypeOneArg(t *testing.T) {
	testParse(t, true, "interface foo.bar\n type I (b:bool)\nmethod F()->()")
}

func TestTypeOneArray(t *testing.T) {
	testParse(t, true, "interface foo.bar\n type I (b:[]bool)\nmethod  F()->()")
	testParse(t, false, "interface foo.bar\n type I (b:bool[ ])\nmethod  F()->()")
	testParse(t, false, "interface foo.bar\n type I (b:bool[1])\nmethod  F()->()")
	testParse(t, false, "interface foo.bar\n type I (b:bool[ 1 ])\nmethod  F()->()")
	testParse(t, false, "interface foo.bar\n type I (b:bool[ 1 1 ])\nmethod  F()->()")
}

func TestFieldnames(t *testing.T) {
	testParse(t, false, "interface foo.bar\n type I (Test:[]bool)\nmethod  F()->()")
	testParse(t, false, "interface foo.bar\n type I (_test:[]bool)\nmethod  F()->()")
	testParse(t, false, "interface foo.bar\n type I (Äest:[]bool)\nmethod  F()->()")
}
func TestNestedStructs(t *testing.T) {
	testParse(t, true, "interface foo.bar\n type I ( b: [](foo: bool, bar: bool, baz: int) )\nmethod  F()->()")
}

func TestEnum(t *testing.T) {
	testParse(t, true, "interface foo.bar\n type I (b:(foo, bar, baz))\nmethod  F()->()")
	testParse(t, false, "interface foo.bar\n type I (foo, bar, baz : bool)\nmethod  F()->()")
}

func TestIncomplete(t *testing.T) {
	testParse(t, false, "interfacef foo.bar\nmethod  F()->()")
	testParse(t, false, "interface foo.bar\nmethod  F()->()\ntype I (b: bool")
	testParse(t, false, "interface foo.bar\nmethod  F()->(")
	testParse(t, false, "interface foo.bar\nmethod  F(")
	testParse(t, false, "interface foo.bar\nmethod  ()->()")
	testParse(t, false, "interface foo.bar\nmethod  F->()\n")
	testParse(t, false, "interface foo.bar\nmethod  F()->\n")
	testParse(t, false, "interface foo.bar\nmethod  F()>()\n")
	testParse(t, false, "interface foo.bar\nmethod  F()->()\ntype (b: bool)")
	testParse(t, false, "interface foo.bar\nmethod  F()->()\nerror (b: bool)")
	testParse(t, false, "interface foo.bar\nmethod  F()->()\n dfghdrg")
}

func TestDuplicate(t *testing.T) {
	testParse(t, false, `
interface foo.example
	type Device()
	type Device()
	type T()
	type T()
	method F() -> ()
	method F() -> ()
`)
}
