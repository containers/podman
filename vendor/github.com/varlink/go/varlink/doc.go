/*
Package varlink provides varlink client and server implementations. See http://varlink.org
for more information about varlink.

Example varlink interface definition in a org.example.this.varlink file:
	interface org.example.this

	method Ping(in: string) -> (out: string)

Generated Go module in a orgexamplethis/orgexamplethis.go file. The generated module
provides reply methods for all methods specified in the varlink interface description.
The stub implementations return a MethodNotImplemented error; the service implementation
using this module will override the methods with its own implementation.
	// Generated with github.com/varlink/go/cmd/varlink-go-interface-generator
	package orgexamplethis

	import "github.com/varlink/go/varlink"

	type orgexamplethisInterface interface {
		Ping(c VarlinkCall, in string) error
	}

	type VarlinkCall struct{ varlink.Call }

	func (c *VarlinkCall) ReplyPing(out string) error {
		var out struct {
			Out string `json:"out,omitempty"`
		}
		out.Out = out
		return c.Reply(&out)
	}

	func (s *VarlinkInterface) Ping(c VarlinkCall, in string) error {
		return c.ReplyMethodNotImplemented("Ping")
	}

	[...]

Service implementing the interface and its method:
	import ("orgexamplethis")

	type Data struct {
		orgexamplethis.VarlinkInterface
		data string
	}

	data := Data{data: "test"}

	func (d *Data) Ping(call orgexamplethis.VarlinkCall, ping string) error {
		return call.ReplyPing(ping)
	}

	service, _ = varlink.NewService(
	        "Example",
	        "This",
	        "1",
	         "https://example.org/this",
	)

	service.RegisterInterface(orgexamplethis.VarlinkNew(&data))
	err := service.Listen("unix:/run/org.example.this", 0)
*/
package varlink
