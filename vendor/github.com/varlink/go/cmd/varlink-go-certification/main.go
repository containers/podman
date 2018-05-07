package main

import (
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/varlink/go/cmd/varlink-go-certification/orgvarlinkcertification"
	"github.com/varlink/go/varlink"
	"io"
	"math"
	"os"
	"strconv"
	"sync"
	"time"
)

func run_client(address string) {
	c, err := varlink.NewConnection(address)
	if err != nil {
		fmt.Println("Failed to connect")
		return
	}
	defer c.Close()

	client_id, err := orgvarlinkcertification.Start().Call(c)
	if err != nil {
		fmt.Println("Start() failed")
		return
	}
	fmt.Printf("Start: '%v'\n", client_id)

	b1, err := orgvarlinkcertification.Test01().Call(c, client_id)
	if err != nil {
		fmt.Println("Test01() failed")
		return
	}
	fmt.Printf("Test01: '%v'\n", b1)

	i2, err := orgvarlinkcertification.Test02().Call(c, client_id, b1)
	if err != nil {
		fmt.Println("Test02() failed")
		return
	}
	fmt.Printf("Test02: '%v'\n", i2)

	f3, err := orgvarlinkcertification.Test03().Call(c, client_id, i2)
	if err != nil {
		fmt.Println("Test03() failed")
		return
	}
	fmt.Printf("Test03: '%v'\n", f3)

	s4, err := orgvarlinkcertification.Test04().Call(c, client_id, f3)
	if err != nil {
		fmt.Println("Test04() failed")
		return
	}
	fmt.Printf("Test04: '%v'\n", s4)

	b5, i5, f5, s5, err := orgvarlinkcertification.Test05().Call(c, client_id, s4)
	if err != nil {
		fmt.Println("Test05() failed")
		return
	}
	fmt.Printf("Test05: '%v'\n", b5)

	o6, err := orgvarlinkcertification.Test06().Call(c, client_id, b5, i5, f5, s5)
	if err != nil {
		fmt.Println("Test06() failed")
		return
	}
	fmt.Printf("Test06: '%v'\n", o6)

	m7, err := orgvarlinkcertification.Test07().Call(c, client_id, o6)
	if err != nil {
		fmt.Println("Test07() failed")
		return
	}
	fmt.Printf("Test07: '%v'\n", m7)

	m8, err := orgvarlinkcertification.Test08().Call(c, client_id, m7)
	if err != nil {
		fmt.Println("Test08() failed")
		return
	}
	fmt.Printf("Test08: '%v'\n", m8)

	t9, err := orgvarlinkcertification.Test09().Call(c, client_id, m8)
	if err != nil {
		fmt.Println("Test09() failed")
		return
	}
	fmt.Printf("Test09: '%v'\n", t9)

	receive10, err := orgvarlinkcertification.Test10().Send(c, varlink.More, client_id, t9)
	if err != nil {
		fmt.Println("Test10() failed")
		return
	}

	fmt.Println("Test10() Send:")
	var a10 []string
	for {
		s10, flags10, err := receive10()
		if err != nil {
			fmt.Println("Test10() receive failed")
			return
		}
		a10 = append(a10, s10)
		fmt.Printf("  Receive: '%v'\n", s10)

		if flags10&varlink.Continues == 0 {
			break
		}
	}
	fmt.Printf("Test10: '%v'\n", a10)

	_, err = orgvarlinkcertification.Test11().Send(c, varlink.Oneway, client_id, a10)
	if err != nil {
		fmt.Println("Test11() failed")
		return
	}
	fmt.Println("Test11: ''")

	end, err := orgvarlinkcertification.End().Call(c, client_id)
	if err != nil {
		fmt.Println("End() failed")
		return
	}
	fmt.Printf("End: '%v'\n", end)
}

// Service
type client struct {
	id   string
	time time.Time
}

type test struct {
	orgvarlinkcertification.VarlinkInterface
	mutex   sync.Mutex
	clients map[string]*client
}

func (t *test) Client(id string) *client {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	return t.clients[id]
}

func (t *test) NewClient() *client {
	id128 := make([]byte, 16)
	io.ReadFull(rand.Reader, id128)
	id128[8] = id128[8]&^0xc0 | 0x80
	id128[6] = id128[6]&^0xf0 | 0x40
	uuid := fmt.Sprintf("%x-%x-%x-%x-%x", id128[0:4], id128[4:6], id128[6:8], id128[8:10], id128[10:])

	t.mutex.Lock()
	defer t.mutex.Unlock()

	// Garbage-collect old clients
	for key, client := range t.clients {
		if time.Since(client.time).Minutes() > 1 {
			delete(t.clients, key)
		}
	}

	if len(t.clients) > 100 {
		return nil
	}

	c := client{
		id:   uuid,
		time: time.Now(),
	}
	t.clients[uuid] = &c

	return &c
}

func (t *test) RemoveClient(id string) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	delete(t.clients, id)
}

func (t *test) Start(c orgvarlinkcertification.VarlinkCall) error {
	return c.ReplyStart(t.NewClient().id)
}

func (t *test) Test01(c orgvarlinkcertification.VarlinkCall, client_id_ string) error {
	if t.Client(client_id_) == nil {
		return c.ReplyClientIdError()
	}

	return c.ReplyTest01(true)
}

func (t *test) Test02(c orgvarlinkcertification.VarlinkCall, client_id_ string, bool_ bool) error {
	if t.Client(client_id_) == nil {
		return c.ReplyClientIdError()
	}

	if !bool_ {
		return c.ReplyCertificationError(nil, nil)
	}

	return c.ReplyTest02(1)
}

func (t *test) Test03(c orgvarlinkcertification.VarlinkCall, client_id_ string, int_ int64) error {
	if t.Client(client_id_) == nil {
		return c.ReplyClientIdError()
	}

	if int_ != 1 {
		return c.ReplyCertificationError(nil, nil)
	}

	return c.ReplyTest03(1.0)
}

func (t *test) Test04(c orgvarlinkcertification.VarlinkCall, client_id_ string, float_ float64) error {
	if t.Client(client_id_) == nil {
		return c.ReplyClientIdError()
	}

	if float_ != 1.0 {
		return c.ReplyCertificationError(nil, nil)
	}

	return c.ReplyTest04("ping")
}
func (t *test) Test05(c orgvarlinkcertification.VarlinkCall, client_id_ string, string_ string) error {
	if t.Client(client_id_) == nil {
		return c.ReplyClientIdError()
	}

	if string_ != "ping" {
		return c.ReplyCertificationError(nil, nil)
	}

	return c.ReplyTest05(false, 2, math.Pi, "a lot of string")
}

func (t *test) Test06(c orgvarlinkcertification.VarlinkCall, client_id_ string, bool_ bool, int_ int64, float_ float64, string_ string) error {
	if t.Client(client_id_) == nil {
		return c.ReplyClientIdError()
	}

	if bool_ {
		return c.ReplyCertificationError(nil, nil)
	}

	if int_ != 2 {
		return c.ReplyCertificationError(nil, nil)
	}

	if float_ != math.Pi {
		return c.ReplyCertificationError(nil, nil)
	}

	if string_ != "a lot of string" {
		return c.ReplyCertificationError(nil, nil)
	}

	s := struct {
		Bool   bool
		Int    int64
		Float  float64
		String string
	}{
		Bool:   false,
		Int:    2,
		Float:  math.Pi,
		String: "a lot of string",
	}
	return c.ReplyTest06(s)
}

func (t *test) Test07(c orgvarlinkcertification.VarlinkCall, client_id_ string, struct_ struct {
	Bool   bool
	Int    int64
	Float  float64
	String string
}) error {
	if t.Client(client_id_) == nil {
		return c.ReplyClientIdError()
	}

	if struct_.Bool {
		return c.ReplyCertificationError(nil, nil)
	}

	if struct_.Int != 2 {
		return c.ReplyCertificationError(nil, nil)
	}

	if struct_.Float != math.Pi {
		return c.ReplyCertificationError(nil, nil)
	}

	if struct_.String != "a lot of string" {
		return c.ReplyCertificationError(nil, nil)
	}

	m := map[string]string{
		"bar": "Bar",
		"foo": "Foo",
	}
	return c.ReplyTest07(m)
}

func (t *test) Test08(c orgvarlinkcertification.VarlinkCall, client_id_ string, map_ map[string]string) error {
	if t.Client(client_id_) == nil {
		return c.ReplyClientIdError()
	}

	if len(map_) != 2 {
		return c.ReplyCertificationError(nil, nil)
	}

	if map_["bar"] != "Bar" {
		return c.ReplyCertificationError(nil, nil)
	}

	if map_["foo"] != "Foo" {
		return c.ReplyCertificationError(nil, nil)
	}

	m := map[string]struct{}{
		"one":   {},
		"two":   {},
		"three": {},
	}
	return c.ReplyTest08(m)
}

func (t *test) Test09(c orgvarlinkcertification.VarlinkCall, client_id_ string, set_ map[string]struct{}) error {
	if t.Client(client_id_) == nil {
		return c.ReplyClientIdError()
	}

	if len(set_) != 3 {
		return c.ReplyCertificationError(nil, nil)
	}

	_, ok := set_["one"]
	if !ok {
		return c.ReplyCertificationError(nil, nil)
	}

	_, ok = set_["two"]
	if !ok {
		return c.ReplyCertificationError(nil, nil)
	}

	_, ok = set_["three"]
	if !ok {
		return c.ReplyCertificationError(nil, nil)
	}

	m := orgvarlinkcertification.MyType{
		Object: json.RawMessage(`{"method": "org.varlink.certification.Test09", "parameters": {"map": {"foo": "Foo", "bar": "Bar"}}}`),
		Enum:   "two",
		Struct: struct {
			First  int64  `json:"first"`
			Second string `json:"second"`
		}{First: 1, Second: "2"},
		Array:                 []string{"one", "two", "three"},
		Dictionary:            map[string]string{"foo": "Foo", "bar": "Bar"},
		Stringset:             map[string]struct{}{"one": {}, "two": {}, "three": {}},
		Nullable:              nil,
		Nullable_array_struct: nil,
		Interface: orgvarlinkcertification.Interface{
			Foo: &[]*map[string]string{
				nil,
				&map[string]string{"Foo": "foo", "Bar": "bar"},
				nil,
				&map[string]string{"one": "foo", "two": "bar"},
			},
			Anon: struct {
				Foo bool `json:"foo"`
				Bar bool `json:"bar"`
			}{Foo: true, Bar: false},
		},
	}
	return c.ReplyTest09(m)
}

func (t *test) Test10(c orgvarlinkcertification.VarlinkCall, client_id_ string, mytype_ orgvarlinkcertification.MyType) error {
	if t.Client(client_id_) == nil {
		return c.ReplyClientIdError()
	}

	var o struct {
		Method     string `json:"method"`
		Parameters struct {
			Map map[string]string `json:"map"`
		} `json:"parameters"`
	}
	err := json.Unmarshal(mytype_.Object, &o)
	if err != nil {
		return err
	}

	if o.Method != "org.varlink.certification.Test09" {
		return c.ReplyCertificationError(nil, nil)
	}

	if len(o.Parameters.Map) != 2 {
		return c.ReplyCertificationError(nil, nil)
	}

	if o.Parameters.Map["bar"] != "Bar" {
		return c.ReplyCertificationError(nil, nil)
	}

	if o.Parameters.Map["foo"] != "Foo" {
		return c.ReplyCertificationError(nil, nil)
	}

	if mytype_.Enum != "two" {
		return c.ReplyCertificationError(nil, nil)
	}

	if mytype_.Struct.First != 1 {
		return c.ReplyCertificationError(nil, nil)
	}

	if mytype_.Struct.Second != "2" {
		return c.ReplyCertificationError(nil, nil)
	}

	if len(mytype_.Array) != 3 {
		return c.ReplyCertificationError(nil, nil)
	}

	if mytype_.Array[0] != "one" && mytype_.Array[1] != "two" && mytype_.Array[2] != "three" {
		return c.ReplyCertificationError(nil, nil)
	}

	if len(mytype_.Dictionary) != 2 {
		return c.ReplyCertificationError(nil, nil)
	}

	if mytype_.Dictionary["bar"] != "Bar" {
		return c.ReplyCertificationError(nil, nil)
	}

	if mytype_.Dictionary["foo"] != "Foo" {
		return c.ReplyCertificationError(nil, nil)
	}

	if len(mytype_.Stringset) != 3 {
		return c.ReplyCertificationError(nil, nil)
	}

	_, ok := mytype_.Stringset["one"]
	if !ok {
		return c.ReplyCertificationError(nil, nil)
	}

	_, ok = mytype_.Stringset["two"]
	if !ok {
		return c.ReplyCertificationError(nil, nil)
	}

	_, ok = mytype_.Stringset["three"]
	if !ok {
		return c.ReplyCertificationError(nil, nil)
	}

	if mytype_.Nullable != nil {
		return c.ReplyCertificationError(nil, nil)
	}

	if mytype_.Nullable_array_struct != nil {
		return c.ReplyCertificationError(nil, nil)
	}

	i := *mytype_.Interface.Foo
	if len(i) != 4 {
		return c.ReplyCertificationError(nil, nil)
	}

	if i[0] != nil {
		return c.ReplyCertificationError(nil, nil)
	}

	if len(*i[1]) != 2 {
		return c.ReplyCertificationError(nil, nil)
	}

	if (*i[1])["Foo"] != "foo" {
		return c.ReplyCertificationError(nil, nil)
	}

	if (*i[1])["Bar"] != "bar" {
		return c.ReplyCertificationError(nil, nil)
	}

	if i[2] != nil {
		return c.ReplyCertificationError(nil, nil)
	}

	if len(*i[3]) != 2 {
		return c.ReplyCertificationError(nil, nil)
	}

	if (*i[3])["one"] != "foo" {
		return c.ReplyCertificationError(nil, nil)
	}

	if (*i[3])["two"] != "bar" {
		return c.ReplyCertificationError(nil, nil)
	}

	if !mytype_.Interface.Anon.Foo {
		return c.ReplyCertificationError(nil, nil)
	}

	if mytype_.Interface.Anon.Bar {
		return c.ReplyCertificationError(nil, nil)
	}

	if !c.WantsMore() {
		return c.ReplyCertificationError(nil, nil)
	}

	for i := 1; i <= 10; i++ {
		c.Continues = i < 10
		err := c.ReplyTest10("Reply number " + strconv.Itoa(i))
		if err != nil {
			return err
		}
	}

	return nil
}

func (t *test) Test11(c orgvarlinkcertification.VarlinkCall, client_id_ string, last_more_replies_ []string) error {
	if t.Client(client_id_) == nil {
		return c.ReplyClientIdError()
	}

	if len(last_more_replies_) != 10 {
		return c.ReplyCertificationError(nil, nil)
	}

	if !c.IsOneway() {
		return c.ReplyCertificationError(nil, nil)
	}

	for i := 1; i <= 10; i++ {
		if last_more_replies_[i] != "Reply number "+strconv.Itoa(i) {
			return c.ReplyCertificationError(nil, nil)
		}
	}

	return c.ReplyTest11()
}

func (t *test) End(c orgvarlinkcertification.VarlinkCall, client_id_ string) error {
	if t.Client(client_id_) == nil {
		return c.ReplyClientIdError()
	}

	t.RemoveClient(client_id_)
	return c.ReplyEnd(true)
}

func run_server(address string) {
	t := test{
		clients: make(map[string]*client),
	}

	s, err := varlink.NewService(
		"Varlink",
		"Certification",
		"1",
		"https://github.com/varlink/go",
	)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	s.RegisterInterface(orgvarlinkcertification.VarlinkNew(&t))
	err = s.Listen(address, 0)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func main() {
	var address string
	var client bool

	flag.StringVar(&address, "varlink", "", "Varlink address")
	flag.BoolVar(&client, "client", false, "Run as client")
	flag.Parse()

	if address == "" {
		flag.Usage()
		os.Exit(1)
	}

	if client {
		run_client(address)
		return
	}

	run_server(address)
}
