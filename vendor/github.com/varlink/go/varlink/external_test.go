package varlink_test

// test with no internal access

import (
	"github.com/varlink/go/varlink"
	"os"
	"runtime"
	"testing"
	"time"
)

type VarlinkInterface struct{}

func (s *VarlinkInterface) VarlinkDispatch(call varlink.Call, methodname string) error {
	return call.ReplyMethodNotImplemented(methodname)
}
func (s *VarlinkInterface) VarlinkGetName() string {
	return `org.example.test`
}

func (s *VarlinkInterface) VarlinkGetDescription() string {
	return "#"
}

type VarlinkInterface2 struct{}

func (s *VarlinkInterface2) VarlinkDispatch(call varlink.Call, methodname string) error {
	return call.ReplyMethodNotImplemented(methodname)
}
func (s *VarlinkInterface2) VarlinkGetName() string {
	return `org.example.test2`
}

func (s *VarlinkInterface2) VarlinkGetDescription() string {
	return "#"
}

func TestRegisterService(t *testing.T) {
	newTestInterface := new(VarlinkInterface)
	service, err := varlink.NewService(
		"Varlink",
		"Varlink Test",
		"1",
		"https://github.com/varlink/go/varlink",
	)
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}

	if err := service.RegisterInterface(newTestInterface); err != nil {
		t.Fatalf("Couldn't register service: %v", err)
	}

	if err := service.RegisterInterface(newTestInterface); err == nil {
		t.Fatal("Could register service twice")
	}

	defer func() { service.Shutdown() }()

	servererror := make(chan error)

	go func() {
		servererror <- service.Listen("unix:varlinkexternal_TestRegisterService", 0)
	}()

	time.Sleep(time.Second / 5)

	n := new(VarlinkInterface2)

	if err := service.RegisterInterface(n); err == nil {
		t.Fatal("Could register service while running")
	}
	time.Sleep(time.Second / 5)
	service.Shutdown()

	if err := <-servererror; err != nil {
		t.Fatalf("service.Listen(): %v", err)
	}
}

func TestUnix(t *testing.T) {
	newTestInterface := new(VarlinkInterface)
	service, err := varlink.NewService(
		"Varlink",
		"Varlink Test",
		"1",
		"https://github.com/varlink/go/varlink",
	)

	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}

	if err := service.RegisterInterface(newTestInterface); err != nil {
		t.Fatalf("RegisterInterface(): %v", err)
	}

	servererror := make(chan error)

	go func() {
		servererror <- service.Listen("unix:varlinkexternal_TestUnix", 0)
	}()

	time.Sleep(time.Second / 5)
	service.Shutdown()

	if err := <-servererror; err != nil {
		t.Fatalf("service.Listen(): %v", err)
	}
}

func TestAnonUnix(t *testing.T) {
	if runtime.GOOS != "linux" {
		return
	}

	newTestInterface := new(VarlinkInterface)
	service, err := varlink.NewService(
		"Varlink",
		"Varlink Test",
		"1",
		"https://github.com/varlink/go/varlink",
	)

	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}

	if err := service.RegisterInterface(newTestInterface); err != nil {
		t.Fatalf("RegisterInterface(): %v", err)
	}

	servererror := make(chan error)

	go func() {
		servererror <- service.Listen("unix:@varlinkexternal_TestAnonUnix", 0)
	}()

	time.Sleep(time.Second / 5)
	service.Shutdown()

	if err := <-servererror; err != nil {
		t.Fatalf("service.Listen(): %v", err)
	}
}

func TestListenFDSNotInt(t *testing.T) {
	newTestInterface := new(VarlinkInterface)
	service, err := varlink.NewService(
		"Varlink",
		"Varlink Test",
		"1",
		"https://github.com/varlink/go/varlink",
	)

	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}

	if err := service.RegisterInterface(newTestInterface); err != nil {
		t.Fatalf("Couldn't register service: %v", err)
	}
	os.Setenv("LISTEN_FDS", "foo")
	os.Setenv("LISTEN_PID", string(os.Getpid()))

	servererror := make(chan error)

	go func() {
		servererror <- service.Listen("unix:varlinkexternal_TestListenFDSNotInt", 0)
	}()

	time.Sleep(time.Second / 5)
	service.Shutdown()

	err = <-servererror

	if err != nil {
		t.Fatalf("service.Run(): %v", err)
	}
}
