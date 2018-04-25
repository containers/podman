package varlink

// tests with access to internals

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func expect(t *testing.T, expected string, returned string) {
	if strings.Compare(returned, expected) != 0 {
		t.Fatalf("Expected(%d): `%s`\nGot(%d): `%s`\n",
			len(expected), expected,
			len(returned), strings.Replace(returned, "\000", "`+\"\\000\"+`", -1))
	}
}

func TestService(t *testing.T) {
	service, _ := NewService(
		"Varlink",
		"Varlink Test",
		"1",
		"https://github.com/varlink/go/varlink",
	)

	t.Run("ZeroMessage", func(t *testing.T) {
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		if err := service.handleMessage(w, []byte{0}); err == nil {
			t.Fatal("HandleMessage returned non-error")
		}
	})

	t.Run("InvalidJson", func(t *testing.T) {
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		msg := []byte(`{"method":"foo.GetInterfaceDescription" fdgdfg}`)
		if err := service.handleMessage(w, msg); err == nil {
			t.Fatal("HandleMessage returned no error on invalid json")
		}
	})

	t.Run("WrongInterface", func(t *testing.T) {
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		msg := []byte(`{"method":"foo.GetInterfaceDescription"}`)
		if err := service.handleMessage(w, msg); err != nil {
			t.Fatal("HandleMessage returned error on wrong interface")
		}
		expect(t, `{"parameters":{"interface":"foo"},"error":"org.varlink.service.InterfaceNotFound"}`+"\000",
			b.String())
	})

	t.Run("InvalidMethod", func(t *testing.T) {
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		msg := []byte(`{"method":"InvalidMethod"}`)
		if err := service.handleMessage(w, msg); err != nil {
			t.Fatal("HandleMessage returned error on invalid method")
		}
		expect(t, `{"parameters":{"parameter":"method"},"error":"org.varlink.service.InvalidParameter"}`+"\000",
			b.String())
	})

	t.Run("WrongMethod", func(t *testing.T) {
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		msg := []byte(`{"method":"org.varlink.service.WrongMethod"}`)
		if err := service.handleMessage(w, msg); err != nil {
			t.Fatal("HandleMessage returned error on wrong method")
		}
		expect(t, `{"parameters":{"method":"WrongMethod"},"error":"org.varlink.service.MethodNotFound"}`+"\000",
			b.String())
	})

	t.Run("GetInterfaceDescriptionNullParameters", func(t *testing.T) {
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		msg := []byte(`{"method":"org.varlink.service.GetInterfaceDescription","parameters": null}`)
		if err := service.handleMessage(w, msg); err != nil {
			t.Fatalf("HandleMessage returned error: %v", err)
		}
		expect(t, `{"parameters":{"parameter":"parameters"},"error":"org.varlink.service.InvalidParameter"}`+"\000",
			b.String())
	})

	t.Run("GetInterfaceDescriptionNoInterface", func(t *testing.T) {
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		msg := []byte(`{"method":"org.varlink.service.GetInterfaceDescription","parameters":{}}`)
		if err := service.handleMessage(w, msg); err != nil {
			t.Fatalf("HandleMessage returned error: %v", err)
		}
		expect(t, `{"parameters":{"parameter":"interface"},"error":"org.varlink.service.InvalidParameter"}`+"\000",
			b.String())
	})

	t.Run("GetInterfaceDescriptionWrongInterface", func(t *testing.T) {
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		msg := []byte(`{"method":"org.varlink.service.GetInterfaceDescription","parameters":{"interface":"foo"}}`)
		if err := service.handleMessage(w, msg); err != nil {
			t.Fatalf("HandleMessage returned error: %v", err)
		}
		expect(t, `{"parameters":{"parameter":"interface"},"error":"org.varlink.service.InvalidParameter"}`+"\000",
			b.String())
	})

	t.Run("GetInterfaceDescription", func(t *testing.T) {
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		msg := []byte(`{"method":"org.varlink.service.GetInterfaceDescription","parameters":{"interface":"org.varlink.service"}}`)
		if err := service.handleMessage(w, msg); err != nil {
			t.Fatalf("HandleMessage returned error: %v", err)
		}
		expect(t, `{"parameters":{"description":"# The Varlink Service Interface is provided by every varlink service. It\n# describes the service and the interfaces it implements.\ninterface org.varlink.service\n\n# Get a list of all the interfaces a service provides and information\n# about the implementation.\nmethod GetInfo() -\u003e (\n  vendor: string,\n  product: string,\n  version: string,\n  url: string,\n  interfaces: []string\n)\n\n# Get the description of an interface that is implemented by this service.\nmethod GetInterfaceDescription(interface: string) -\u003e (description: string)\n\n# The requested interface was not found.\nerror InterfaceNotFound (interface: string)\n\n# The requested method was not found\nerror MethodNotFound (method: string)\n\n# The interface defines the requested method, but the service does not\n# implement it.\nerror MethodNotImplemented (method: string)\n\n# One of the passed parameters is invalid.\nerror InvalidParameter (parameter: string)"}}`+"\000",
			b.String())
	})

	t.Run("GetInfo", func(t *testing.T) {
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		msg := []byte(`{"method":"org.varlink.service.GetInfo"}`)
		if err := service.handleMessage(w, msg); err != nil {
			t.Fatalf("HandleMessage returned error: %v", err)
		}
		expect(t, `{"parameters":{"vendor":"Varlink","product":"Varlink Test","version":"1","url":"https://github.com/varlink/go/varlink","interfaces":["org.varlink.service"]}}`+"\000",
			b.String())
	})
}

type VarlinkInterface struct{}

func (s *VarlinkInterface) VarlinkDispatch(call Call, methodname string) error {
	switch methodname {
	case "Ping":
		if !call.WantsMore() {
			return fmt.Errorf("More flag not passed")
		}
		if call.IsOneShot() {
			return fmt.Errorf("OneShot flag set")
		}
		call.Continues = true
		if err := call.Reply(nil); err != nil {
			return err
		}
		if err := call.Reply(nil); err != nil {
			return err
		}
		call.Continues = false
		if err := call.Reply(nil); err != nil {
			return err
		}
		return nil

	case "PingError":
		return call.ReplyError("org.example.test.PingError", nil)
	}

	call.Continues = true
	if err := call.Reply(nil); err == nil {
		return fmt.Errorf("call.Reply did not fail for Continues/More mismatch")
	}
	call.Continues = false

	if err := call.ReplyError("WrongName", nil); err == nil {
		return fmt.Errorf("call.ReplyError accepted invalid error name")
	}

	if err := call.ReplyError("org.varlink.service.MethodNotImplemented", nil); err == nil {
		return fmt.Errorf("call.ReplyError accepted org.varlink.service error")
	}

	return call.ReplyMethodNotImplemented(methodname)
}
func (s *VarlinkInterface) VarlinkGetName() string {
	return `org.example.test`
}

func (s *VarlinkInterface) VarlinkGetDescription() string {
	return "#"
}

func TestMoreService(t *testing.T) {
	newTestInterface := new(VarlinkInterface)

	service, _ := NewService(
		"Varlink",
		"Varlink Test",
		"1",
		"https://github.com/varlink/go/varlink",
	)

	if err := service.RegisterInterface(newTestInterface); err != nil {
		t.Fatalf("Couldn't register service: %v", err)
	}

	t.Run("MethodNotImplemented", func(t *testing.T) {
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		msg := []byte(`{"method":"org.example.test.Pingf"}`)
		if err := service.handleMessage(w, msg); err != nil {
			t.Fatalf("HandleMessage returned error: %v", err)
		}
		expect(t, `{"parameters":{"method":"Pingf"},"error":"org.varlink.service.MethodNotImplemented"}`+"\000",
			b.String())
	})

	t.Run("PingError", func(t *testing.T) {
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		msg := []byte(`{"method":"org.example.test.PingError", "more" : true}`)
		if err := service.handleMessage(w, msg); err != nil {
			t.Fatalf("HandleMessage returned error: %v", err)
		}
		expect(t, `{"error":"org.example.test.PingError"}`+"\000",
			b.String())
	})
	t.Run("MoreTest", func(t *testing.T) {
		var b bytes.Buffer
		w := bufio.NewWriter(&b)
		msg := []byte(`{"method":"org.example.test.Ping", "more" : true}`)
		if err := service.handleMessage(w, msg); err != nil {
			t.Fatalf("HandleMessage returned error: %v", err)
		}
		expect(t, `{"continues":true}`+"\000"+`{"continues":true}`+"\000"+`{}`+"\000",
			b.String())
	})
}
