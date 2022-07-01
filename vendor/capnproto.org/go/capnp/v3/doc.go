/*
Package capnp is a Cap'n Proto library for Go.
https://capnproto.org/

Read the Getting Started guide for a tutorial on how to use this
package. https://github.com/capnproto/go-capnproto2/wiki/Getting-Started

Generating code

capnpc-go provides the compiler backend for capnp.

	# First, install capnpc-go to $PATH.
	go install capnproto.org/go/capnp/v3/capnpc-go
	# Then, generate Go files.
	capnp compile -I$GOPATH/src/capnproto.org/go/capnp/v3/std -ogo *.capnp

capnpc-go requires two annotations for all files: package and import.
package is needed to know what package to place at the head of the
generated file and what identifier to use when referring to the type
from another package.  import should be the fully qualified import path
and is used to generate import statement from other packages and to
detect when two types are in the same package.  For example:

	using Go = import "/go.capnp";
	$Go.package("main");
	$Go.import("capnproto.org/go/capnp/v3/example");

For adding documentation comments to the generated code, there's the doc
annotation. This annotation adds the comment to a struct, enum or field so
that godoc will pick it up. For example:

	struct Zdate $Go.doc("Zdate represents a calendar date") {
	  year  @0   :Int16;
	  month @1   :UInt8;
	  day   @2   :UInt8 ;
	}

Messages and Segments

In Cap'n Proto, the unit of communication is a message. A message
consists of one or more segments -- contiguous blocks of memory.  This
allows large messages to be split up and loaded independently or lazily.
Typically you will use one segment per message.  Logically, a message is
organized in a tree of objects, with the root always being a struct (as
opposed to a list or primitive).  Messages can be read from and written
to a stream.

The Message and Segment types are the main types that application code
will use from this package.  The Message type has methods for marshaling
and unmarshaling its segments to the wire format.  If the application
needs to read or write from a stream, it should use the Encoder and
Decoder types.

Pointers

The type for a generic reference to a Cap'n Proto object is Ptr.  A Ptr
can refer to a struct, a list, or an interface.  Ptr, Struct, List, and
Interface (the pointer types) have value semantics and refer to data in
a single segment.  All of the pointer types have a notion of "valid".
An invalid pointer will return the default value from any accessor and
panic when any setter is called.

In previous versions of this package, the Pointer interface was used
instead of the Ptr struct.  This interface and functions that use it are
now deprecated.  See https://github.com/capnproto/go-capnproto2/wiki/New-Ptr-Type
for details about this API change.

Data accessors and setters (i.e. struct primitive fields and list
elements) do not return errors, but pointer accessors and setters do.
There are a few reasons that a read or write of a pointer can fail, but
the most common are bad pointers or allocation failures.  For accessors,
an invalid object will be returned in case of an error.

Since Go doesn't have generics, wrapper types provide type safety on
lists.  This package provides lists of basic types, and capnpc-go
generates list wrappers for named types.  However, if you need to use
deeper nesting of lists (e.g. List(List(UInt8))), you will need to use a
PointerList and wrap the elements.

Structs

For the following schema:

struct Foo @0x8423424e9b01c0af {
  num @0 :UInt32;
  bar @1 :Foo;
}

capnpc-go will generate:

	// Foo is a pointer to a Foo struct in a segment.
	// Member functions are provided to get/set members in the
	// struct.
	type Foo struct{ capnp.Struct }

	// Foo_TypeID is the unique identifier for the type Foo.
	// It remains the same across languages and schema changes.
	const Foo_TypeID = 0x8423424e9b01c0af

	// NewFoo creates a new orphaned Foo struct, preferring placement in
	// s.  If there isn't enough space, then another segment in the
	// message will be used or allocated.  You can set a field of type Foo
	// to this new message, but usually you will want to use the
	// NewBar()-style method shown below.
	func NewFoo(s *capnp.Segment) (Foo, error)

	// NewRootFoo creates a new Foo struct and sets the message's root to
	// it.
	func NewRootFoo(s *capnp.Segment) (Foo, error)

	// ReadRootFoo reads the message's root pointer and converts it to a
	// Foo struct.
	func ReadRootFoo(msg *capnp.Message) (Foo, error)

	// Num returns the value of the num field.
	func (s Foo) Num() uint32

	// SetNum sets the value of the num field to v.
	func (s Foo) SetNum(v uint32)

	// Bar returns the value of the bar field.  This can return an error
	// if the pointer goes beyond the segment's range, the segment fails
	// to load, or the pointer recursion limit has been reached.
	func (s Foo) Bar() (Foo, error)

	// HasBar reports whether the bar field was initialized (non-null).
	func (s Foo) HasBar() bool

	// SetBar sets the value of the bar field to v.
	func (s Foo) SetBar(v Foo) error

	// NewBar sets the bar field to a newly allocated Foo struct,
	// preferring placement in s's segment.
	func (s Foo) NewBar() (Foo, error)

	// Foo_List is a value with pointer semantics. It is created for all
	// structs, and is used for List(Foo) in the capnp file.
	type Foo_List struct{ capnp.List }

	// NewFoo_List creates a new orphaned List(Foo), preferring placement
	// in s. This can then be added to a message by using a Set function
	// which takes a Foo_List. sz specifies the number of elements in the
	// list.  The list's size cannot be changed after creation.
	func NewFoo_List(s *capnp.Segment, sz int32) Foo_List

	// Len returns the number of elements in the list.
	func (s Foo_List) Len() int

	// At returns a pointer to the i'th element. If i is an invalid index,
	// this will return an invalid Foo (all getters will return default
	// values, setters will fail).
	func (s Foo_List) At(i int) Foo

	// Foo_Promise is a promise for a Foo.  Methods are provided to get
	// promises of struct and interface fields.
	type Foo_Promise struct{ *capnp.Pipeline }

	// Get waits until the promise is resolved and returns the result.
	func (p Foo_Promise) Get() (Foo, error)

	// Bar returns a promise for that bar field.
	func (p Foo_Promise) Bar() Foo_Promise


Groups

For each group a typedef is created with a different method set for just the
groups fields:

	struct Foo {
		group :Group {
			field @0 :Bool;
		}
	}

generates the following:

	type Foo struct{ capnp.Struct }
	type Foo_group Foo

	func (s Foo) Group() Foo_group
	func (s Foo_group) Field() bool

That way the following may be used to access a field in a group:

	var f Foo
	value := f.Group().Field()

Note that group accessors just convert the type and so have no overhead.

Unions

Named unions are treated as a group with an inner unnamed union. Unnamed
unions generate an enum Type_Which and a corresponding Which() function:

	struct Foo {
		union {
			a @0 :Bool;
			b @1 :Bool;
		}
	}

generates the following:

	type Foo_Which uint16

	const (
		Foo_Which_a Foo_Which = 0
		Foo_Which_b Foo_Which = 1
	)

	func (s Foo) A() bool
	func (s Foo) B() bool
	func (s Foo) SetA(v bool)
	func (s Foo) SetB(v bool)
	func (s Foo) Which() Foo_Which

Which() should be checked before using the getters, and the default case must
always be handled.

Setters for single values will set the union discriminator as well as set the
value.

For voids in unions, there is a void setter that just sets the discriminator.
For example:

	struct Foo {
		union {
			a @0 :Void;
			b @1 :Void;
		}
	}

generates the following:

	func (s Foo) SetA() // Set that we are using A
	func (s Foo) SetB() // Set that we are using B

Similarly, for groups in unions, there is a group setter that just sets
the discriminator. This must be called before the group getter can be
used to set values. For example:

	struct Foo {
		union {
			a :group {
				v :Bool
			}
			b :group {
				v :Bool
			}
		}
	}

and in usage:

	f.SetA()         // Set that we are using group A
	f.A().SetV(true) // then we can use the group A getter to set the inner values

Enums

capnpc-go generates enum values as constants. For example in the capnp file:

	enum ElementSize {
	  empty @0;
	  bit @1;
	  byte @2;
	  twoBytes @3;
	  fourBytes @4;
	  eightBytes @5;
	  pointer @6;
	  inlineComposite @7;
	}

In the generated capnp.go file:

	type ElementSize uint16

	const (
		ElementSize_empty           ElementSize = 0
		ElementSize_bit             ElementSize = 1
		ElementSize_byte            ElementSize = 2
		ElementSize_twoBytes        ElementSize = 3
		ElementSize_fourBytes       ElementSize = 4
		ElementSize_eightBytes      ElementSize = 5
		ElementSize_pointer         ElementSize = 6
		ElementSize_inlineComposite ElementSize = 7
	)

In addition an enum.String() function is generated that will convert the constants to a string
for debugging or logging purposes. By default, the enum name is used as the tag value,
but the tags can be customized with a $Go.tag or $Go.notag annotation.

For example:

	enum ElementSize {
		empty @0           $Go.tag("void");
		bit @1             $Go.tag("1 bit");
		byte @2            $Go.tag("8 bits");
		inlineComposite @7 $Go.notag;
	}

In the generated go file:

	func (c ElementSize) String() string {
		switch c {
		case ElementSize_empty:
			return "void"
		case ElementSize_bit:
			return "1 bit"
		case ElementSize_byte:
			return "8 bits"
		default:
			return ""
		}
	}

Interfaces

capnpc-go generates type-safe Client wrappers for interfaces. For parameter
lists and result lists, structs are generated as described above with the names
Interface_method_Params and Interface_method_Results, unless a single struct
type is used. For example, for this interface:

	interface Calculator {
		evaluate @0 (expression :Expression) -> (value :Value);
	}

capnpc-go generates the following Go code (along with the structs
Calculator_evaluate_Params and Calculator_evaluate_Results):

	// Calculator is a client to a Calculator interface.
	type Calculator struct{ Client capnp.Client }

	// Evaluate calls `evaluate` on the client.  params is called on a newly
	// allocated Calculator_evaluate_Params struct to fill in the parameters.
	func (c Calculator) Evaluate(
		ctx context.Context,
		params func(Calculator_evaluate_Params) error,
		opts ...capnp.CallOption) *Calculator_evaluate_Results_Promise

capnpc-go also generates code to implement the interface:

	// A Calculator_Server implements the Calculator interface.
	type Calculator_Server interface {
		Evaluate(context.Context, Calculator_evaluate_Call) error
	}

	// Calculator_evaluate_Call holds the arguments for a Calculator.evaluate server call.
	type Calculator_evaluate_Call struct {
		Params  Calculator_evaluate_Params
		Results Calculator_evaluate_Results
		Options capnp.CallOptions
	}

	// Calculator_ServerToClient is equivalent to calling:
	// NewCalculator(capnp.NewServer(Calculator_Methods(nil, s), s))
	// If s does not implement the Close method, then nil is used.
	func Calculator_ServerToClient(s Calculator_Server) Calculator

	// Calculator_Methods appends methods from Calculator that call to server and
	// returns the methods.  If methods is nil or the capacity of the underlying
	// slice is too small, a new slice is returned.
	func Calculator_Methods(methods []server.Method, s Calculator_Server) []server.Method

Since a single capability may want to implement many interfaces, you can
use multiple *_Methods functions to build a single slice to send to
NewServer.

An example of combining the client/server code to communicate with a locally
implemented Calculator:

	var srv Calculator_Server
	calc := Calculator_ServerToClient(srv)
	result := calc.Evaluate(ctx, func(params Calculator_evaluate_Params) {
		params.SetExpression(expr)
	})
	val := result.Value().Get()

A note about message ordering: when implementing a server method, you
are responsible for acknowledging delivery of a method call.  Failure to
do so can cause deadlocks.  See the server.Ack function for more details.
*/
package capnp // import "capnproto.org/go/capnp/v3"
