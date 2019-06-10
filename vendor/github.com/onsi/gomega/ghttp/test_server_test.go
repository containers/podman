package ghttp_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"

	"github.com/golang/protobuf/proto"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp/protobuf"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/ghttp"
)

var _ = Describe("TestServer", func() {
	var (
		resp *http.Response
		err  error
		s    *Server
	)

	BeforeEach(func() {
		s = NewServer()
	})

	AfterEach(func() {
		s.Close()
	})

	Describe("Resetting the server", func() {
		BeforeEach(func() {
			s.RouteToHandler("GET", "/", func(w http.ResponseWriter, req *http.Request) {})
			s.AppendHandlers(func(w http.ResponseWriter, req *http.Request) {})
			http.Get(s.URL() + "/")

			Expect(s.ReceivedRequests()).Should(HaveLen(1))
		})

		It("clears all handlers and call counts", func() {
			s.Reset()
			Expect(s.ReceivedRequests()).Should(HaveLen(0))
			Expect(func() { s.GetHandler(0) }).Should(Panic())
		})
	})

	Describe("closing client connections", func() {
		It("closes", func() {
			s.RouteToHandler("GET", "/",
				func(w http.ResponseWriter, req *http.Request) {
					io.WriteString(w, req.RemoteAddr)
				},
			)
			client := http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
			resp, err := client.Get(s.URL())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).Should(Equal(200))

			body, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			Expect(err).ShouldNot(HaveOccurred())

			s.CloseClientConnections()

			resp, err = client.Get(s.URL())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).Should(Equal(200))

			body2, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			Expect(err).ShouldNot(HaveOccurred())

			Expect(body2).ShouldNot(Equal(body))
		})
	})

	Describe("closing server mulitple times", func() {
		It("should not fail", func() {
			s.Close()
			Expect(s.Close).ShouldNot(Panic())
		})
	})

	Describe("allowing unhandled requests", func() {
		It("is not permitted by default", func() {
			Expect(s.GetAllowUnhandledRequests()).To(BeFalse())
		})

		Context("when true", func() {
			BeforeEach(func() {
				s.SetAllowUnhandledRequests(true)
				s.SetUnhandledRequestStatusCode(http.StatusForbidden)
				resp, err = http.Get(s.URL() + "/foo")
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("should allow unhandled requests and respond with the passed in status code", func() {
				Expect(err).ShouldNot(HaveOccurred())
				Expect(resp.StatusCode).Should(Equal(http.StatusForbidden))

				data, err := ioutil.ReadAll(resp.Body)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(data).Should(BeEmpty())
			})

			It("should record the requests", func() {
				Expect(s.ReceivedRequests()).Should(HaveLen(1))
				Expect(s.ReceivedRequests()[0].URL.Path).Should(Equal("/foo"))
			})
		})

		Context("when false", func() {
			It("should fail when attempting a request", func() {
				failures := InterceptGomegaFailures(func() {
					http.Get(s.URL() + "/foo")
				})

				Expect(failures[0]).Should(ContainSubstring("Received Unhandled Request"))
			})
		})
	})

	Describe("Managing Handlers", func() {
		var called []string
		BeforeEach(func() {
			called = []string{}
			s.RouteToHandler("GET", "/routed", func(w http.ResponseWriter, req *http.Request) {
				called = append(called, "r1")
			})
			s.RouteToHandler("POST", regexp.MustCompile(`/routed\d`), func(w http.ResponseWriter, req *http.Request) {
				called = append(called, "r2")
			})
			s.AppendHandlers(func(w http.ResponseWriter, req *http.Request) {
				called = append(called, "A")
			}, func(w http.ResponseWriter, req *http.Request) {
				called = append(called, "B")
			})
		})

		It("should prefer routed handlers if there is a match", func() {
			http.Get(s.URL() + "/routed")
			http.Post(s.URL()+"/routed7", "application/json", nil)
			http.Get(s.URL() + "/foo")
			http.Get(s.URL() + "/routed")
			http.Post(s.URL()+"/routed9", "application/json", nil)
			http.Get(s.URL() + "/bar")

			failures := InterceptGomegaFailures(func() {
				http.Get(s.URL() + "/foo")
				http.Get(s.URL() + "/routed/not/a/match")
				http.Get(s.URL() + "/routed7")
				http.Post(s.URL()+"/routed", "application/json", nil)
			})

			Expect(failures[0]).Should(ContainSubstring("Received Unhandled Request"))
			Expect(failures).Should(HaveLen(4))

			http.Post(s.URL()+"/routed3", "application/json", nil)

			Expect(called).Should(Equal([]string{"r1", "r2", "A", "r1", "r2", "B", "r2"}))
		})

		It("should override routed handlers when reregistered", func() {
			s.RouteToHandler("GET", "/routed", func(w http.ResponseWriter, req *http.Request) {
				called = append(called, "r3")
			})
			s.RouteToHandler("POST", regexp.MustCompile(`/routed\d`), func(w http.ResponseWriter, req *http.Request) {
				called = append(called, "r4")
			})

			http.Get(s.URL() + "/routed")
			http.Post(s.URL()+"/routed7", "application/json", nil)

			Expect(called).Should(Equal([]string{"r3", "r4"}))
		})

		It("should call the appended handlers, in order, as requests come in", func() {
			http.Get(s.URL() + "/foo")
			Expect(called).Should(Equal([]string{"A"}))

			http.Get(s.URL() + "/foo")
			Expect(called).Should(Equal([]string{"A", "B"}))

			failures := InterceptGomegaFailures(func() {
				http.Get(s.URL() + "/foo")
			})

			Expect(failures[0]).Should(ContainSubstring("Received Unhandled Request"))
		})

		Describe("Overwriting an existing handler", func() {
			BeforeEach(func() {
				s.SetHandler(0, func(w http.ResponseWriter, req *http.Request) {
					called = append(called, "C")
				})
			})

			It("should override the specified handler", func() {
				http.Get(s.URL() + "/foo")
				http.Get(s.URL() + "/foo")
				Expect(called).Should(Equal([]string{"C", "B"}))
			})
		})

		Describe("Getting an existing handler", func() {
			It("should return the handler func", func() {
				s.GetHandler(1)(nil, nil)
				Expect(called).Should(Equal([]string{"B"}))
			})
		})

		Describe("Wrapping an existing handler", func() {
			BeforeEach(func() {
				s.WrapHandler(0, func(w http.ResponseWriter, req *http.Request) {
					called = append(called, "C")
				})
			})

			It("should wrap the existing handler in a new handler", func() {
				http.Get(s.URL() + "/foo")
				http.Get(s.URL() + "/foo")
				Expect(called).Should(Equal([]string{"A", "C", "B"}))
			})
		})
	})

	Describe("When a handler fails", func() {
		BeforeEach(func() {
			s.SetUnhandledRequestStatusCode(http.StatusForbidden) //just to be clear that 500s aren't coming from unhandled requests
		})

		Context("because the handler has panicked", func() {
			BeforeEach(func() {
				s.AppendHandlers(func(w http.ResponseWriter, req *http.Request) {
					panic("bam")
				})
			})

			It("should respond with a 500 and make a failing assertion", func() {
				var resp *http.Response
				var err error

				failures := InterceptGomegaFailures(func() {
					resp, err = http.Get(s.URL())
				})

				Expect(err).ShouldNot(HaveOccurred())
				Expect(resp.StatusCode).Should(Equal(http.StatusInternalServerError))
				Expect(failures).Should(ConsistOf(ContainSubstring("Handler Panicked")))
			})
		})

		Context("because an assertion has failed", func() {
			BeforeEach(func() {
				s.AppendHandlers(func(w http.ResponseWriter, req *http.Request) {
					// Expect(true).Should(BeFalse()) <-- would be nice to do it this way, but the test just can't be written this way

					By("We're cheating a bit here -- we're throwing a GINKGO_PANIC which simulates a failed assertion")
					panic(GINKGO_PANIC)
				})
			})

			It("should respond with a 500 and *not* make a failing assertion, instead relying on Ginkgo to have already been notified of the error", func() {
				resp, err := http.Get(s.URL())

				Expect(err).ShouldNot(HaveOccurred())
				Expect(resp.StatusCode).Should(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("Logging to the Writer", func() {
		var buf *gbytes.Buffer
		BeforeEach(func() {
			buf = gbytes.NewBuffer()
			s.Writer = buf
			s.AppendHandlers(func(w http.ResponseWriter, req *http.Request) {})
			s.AppendHandlers(func(w http.ResponseWriter, req *http.Request) {})
		})

		It("should write to the buffer when a request comes in", func() {
			http.Get(s.URL() + "/foo")
			Expect(buf).Should(gbytes.Say("GHTTP Received Request: GET - /foo\n"))

			http.Post(s.URL()+"/bar", "", nil)
			Expect(buf).Should(gbytes.Say("GHTTP Received Request: POST - /bar\n"))
		})
	})

	Describe("Request Handlers", func() {
		Describe("VerifyRequest", func() {
			BeforeEach(func() {
				s.AppendHandlers(VerifyRequest("GET", "/foo"))
			})

			It("should verify the method, path", func() {
				resp, err = http.Get(s.URL() + "/foo?baz=bar")
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("should verify the method, path", func() {
				failures := InterceptGomegaFailures(func() {
					http.Get(s.URL() + "/foo2")
				})
				Expect(failures).Should(HaveLen(1))
			})

			It("should verify the method, path", func() {
				failures := InterceptGomegaFailures(func() {
					http.Post(s.URL()+"/foo", "application/json", nil)
				})
				Expect(failures).Should(HaveLen(1))
			})

			Context("when passed a rawQuery", func() {
				It("should also be possible to verify the rawQuery", func() {
					s.SetHandler(0, VerifyRequest("GET", "/foo", "baz=bar"))
					resp, err = http.Get(s.URL() + "/foo?baz=bar")
					Expect(err).ShouldNot(HaveOccurred())
				})

				It("should match irregardless of query parameter ordering", func() {
					s.SetHandler(0, VerifyRequest("GET", "/foo", "type=get&name=money"))
					u, _ := url.Parse(s.URL() + "/foo")
					u.RawQuery = url.Values{
						"type": []string{"get"},
						"name": []string{"money"},
					}.Encode()

					resp, err = http.Get(u.String())
					Expect(err).ShouldNot(HaveOccurred())
				})
			})

			Context("when passed a matcher for path", func() {
				It("should apply the matcher", func() {
					s.SetHandler(0, VerifyRequest("GET", MatchRegexp(`/foo/[a-f]*/3`)))
					resp, err = http.Get(s.URL() + "/foo/abcdefa/3")
					Expect(err).ShouldNot(HaveOccurred())
				})
			})
		})

		Describe("VerifyContentType", func() {
			BeforeEach(func() {
				s.AppendHandlers(CombineHandlers(
					VerifyRequest("GET", "/foo"),
					VerifyContentType("application/octet-stream"),
				))
			})

			It("should verify the content type", func() {
				req, err := http.NewRequest("GET", s.URL()+"/foo", nil)
				Expect(err).ShouldNot(HaveOccurred())
				req.Header.Set("Content-Type", "application/octet-stream")

				resp, err = http.DefaultClient.Do(req)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("should verify the content type", func() {
				req, err := http.NewRequest("GET", s.URL()+"/foo", nil)
				Expect(err).ShouldNot(HaveOccurred())
				req.Header.Set("Content-Type", "application/json")

				failures := InterceptGomegaFailures(func() {
					http.DefaultClient.Do(req)
				})
				Expect(failures).Should(HaveLen(1))
			})

			It("should verify the content type", func() {
				req, err := http.NewRequest("GET", s.URL()+"/foo", nil)
				Expect(err).ShouldNot(HaveOccurred())
				req.Header.Set("Content-Type", "application/octet-stream; charset=utf-8")

				failures := InterceptGomegaFailures(func() {
					http.DefaultClient.Do(req)
				})
				Expect(failures).Should(HaveLen(1))
			})
		})

		Describe("Verify BasicAuth", func() {
			BeforeEach(func() {
				s.AppendHandlers(CombineHandlers(
					VerifyRequest("GET", "/foo"),
					VerifyBasicAuth("bob", "password"),
				))
			})

			It("should verify basic auth", func() {
				req, err := http.NewRequest("GET", s.URL()+"/foo", nil)
				Expect(err).ShouldNot(HaveOccurred())
				req.SetBasicAuth("bob", "password")

				resp, err = http.DefaultClient.Do(req)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("should verify basic auth", func() {
				req, err := http.NewRequest("GET", s.URL()+"/foo", nil)
				Expect(err).ShouldNot(HaveOccurred())
				req.SetBasicAuth("bob", "bassword")

				failures := InterceptGomegaFailures(func() {
					http.DefaultClient.Do(req)
				})
				Expect(failures).Should(HaveLen(1))
			})

			It("should require basic auth header", func() {
				req, err := http.NewRequest("GET", s.URL()+"/foo", nil)
				Expect(err).ShouldNot(HaveOccurred())

				failures := InterceptGomegaFailures(func() {
					http.DefaultClient.Do(req)
				})
				Expect(failures).Should(ContainElement(ContainSubstring("Authorization header must be specified")))
			})
		})

		Describe("VerifyHeader", func() {
			BeforeEach(func() {
				s.AppendHandlers(CombineHandlers(
					VerifyRequest("GET", "/foo"),
					VerifyHeader(http.Header{
						"accept":        []string{"jpeg", "png"},
						"cache-control": []string{"omicron"},
						"Return-Path":   []string{"hobbiton"},
					}),
				))
			})

			It("should verify the headers", func() {
				req, err := http.NewRequest("GET", s.URL()+"/foo", nil)
				Expect(err).ShouldNot(HaveOccurred())
				req.Header.Add("Accept", "jpeg")
				req.Header.Add("Accept", "png")
				req.Header.Add("Cache-Control", "omicron")
				req.Header.Add("return-path", "hobbiton")

				resp, err = http.DefaultClient.Do(req)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("should verify the headers", func() {
				req, err := http.NewRequest("GET", s.URL()+"/foo", nil)
				Expect(err).ShouldNot(HaveOccurred())
				req.Header.Add("Schmaccept", "jpeg")
				req.Header.Add("Schmaccept", "png")
				req.Header.Add("Cache-Control", "omicron")
				req.Header.Add("return-path", "hobbiton")

				failures := InterceptGomegaFailures(func() {
					http.DefaultClient.Do(req)
				})
				Expect(failures).Should(HaveLen(1))
			})
		})

		Describe("VerifyHeaderKV", func() {
			BeforeEach(func() {
				s.AppendHandlers(CombineHandlers(
					VerifyRequest("GET", "/foo"),
					VerifyHeaderKV("accept", "jpeg", "png"),
					VerifyHeaderKV("cache-control", "omicron"),
					VerifyHeaderKV("Return-Path", "hobbiton"),
				))
			})

			It("should verify the headers", func() {
				req, err := http.NewRequest("GET", s.URL()+"/foo", nil)
				Expect(err).ShouldNot(HaveOccurred())
				req.Header.Add("Accept", "jpeg")
				req.Header.Add("Accept", "png")
				req.Header.Add("Cache-Control", "omicron")
				req.Header.Add("return-path", "hobbiton")

				resp, err = http.DefaultClient.Do(req)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("should verify the headers", func() {
				req, err := http.NewRequest("GET", s.URL()+"/foo", nil)
				Expect(err).ShouldNot(HaveOccurred())
				req.Header.Add("Accept", "jpeg")
				req.Header.Add("Cache-Control", "omicron")
				req.Header.Add("return-path", "hobbiton")

				failures := InterceptGomegaFailures(func() {
					http.DefaultClient.Do(req)
				})
				Expect(failures).Should(HaveLen(1))
			})
		})

		Describe("VerifyBody", func() {
			BeforeEach(func() {
				s.AppendHandlers(CombineHandlers(
					VerifyRequest("POST", "/foo"),
					VerifyBody([]byte("some body")),
				))
			})

			It("should verify the body", func() {
				resp, err = http.Post(s.URL()+"/foo", "", bytes.NewReader([]byte("some body")))
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("should verify the body", func() {
				failures := InterceptGomegaFailures(func() {
					http.Post(s.URL()+"/foo", "", bytes.NewReader([]byte("wrong body")))
				})
				Expect(failures).Should(HaveLen(1))
			})
		})

		Describe("VerifyMimeType", func() {
			BeforeEach(func() {
				s.AppendHandlers(CombineHandlers(
					VerifyMimeType("application/json"),
				))
			})

			It("should verify the mime type in content-type header", func() {
				resp, err = http.Post(s.URL()+"/foo", "application/json; charset=utf-8", bytes.NewReader([]byte(`{}`)))
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("should verify the mime type in content-type header", func() {
				failures := InterceptGomegaFailures(func() {
					http.Post(s.URL()+"/foo", "text/plain", bytes.NewReader([]byte(`{}`)))
				})
				Expect(failures).Should(HaveLen(1))
			})
		})

		Describe("VerifyJSON", func() {
			BeforeEach(func() {
				s.AppendHandlers(CombineHandlers(
					VerifyRequest("POST", "/foo"),
					VerifyJSON(`{"a":3, "b":2}`),
				))
			})

			It("should verify the json body and the content type", func() {
				resp, err = http.Post(s.URL()+"/foo", "application/json", bytes.NewReader([]byte(`{"b":2, "a":3}`)))
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("should verify the json body and the content type", func() {
				failures := InterceptGomegaFailures(func() {
					http.Post(s.URL()+"/foo", "application/json", bytes.NewReader([]byte(`{"b":2, "a":4}`)))
				})
				Expect(failures).Should(HaveLen(1))
			})

			It("should verify the json body and the content type", func() {
				failures := InterceptGomegaFailures(func() {
					http.Post(s.URL()+"/foo", "application/not-json", bytes.NewReader([]byte(`{"b":2, "a":3}`)))
				})
				Expect(failures).Should(HaveLen(1))
			})

			It("should verify the json body and the content type", func() {
				resp, err = http.Post(s.URL()+"/foo", "application/json; charset=utf-8", bytes.NewReader([]byte(`{"b":2, "a":3}`)))
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		Describe("VerifyJSONRepresenting", func() {
			BeforeEach(func() {
				s.AppendHandlers(CombineHandlers(
					VerifyRequest("POST", "/foo"),
					VerifyJSONRepresenting([]int{1, 3, 5}),
				))
			})

			It("should verify the json body and the content type", func() {
				resp, err = http.Post(s.URL()+"/foo", "application/json", bytes.NewReader([]byte(`[1,3,5]`)))
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("should verify the json body and the content type", func() {
				failures := InterceptGomegaFailures(func() {
					http.Post(s.URL()+"/foo", "application/json", bytes.NewReader([]byte(`[1,3]`)))
				})
				Expect(failures).Should(HaveLen(1))
			})
		})

		Describe("VerifyForm", func() {
			var formValues url.Values

			BeforeEach(func() {
				formValues = make(url.Values)
				formValues.Add("users", "user1")
				formValues.Add("users", "user2")
				formValues.Add("group", "users")
			})

			Context("when encoded in the URL", func() {
				BeforeEach(func() {
					s.AppendHandlers(CombineHandlers(
						VerifyRequest("GET", "/foo"),
						VerifyForm(url.Values{
							"users": []string{"user1", "user2"},
							"group": []string{"users"},
						}),
					))
				})

				It("should verify form values", func() {
					resp, err = http.Get(s.URL() + "/foo?" + formValues.Encode())
					Expect(err).ShouldNot(HaveOccurred())
				})

				It("should ignore extra values", func() {
					formValues.Add("extra", "value")
					resp, err = http.Get(s.URL() + "/foo?" + formValues.Encode())
					Expect(err).ShouldNot(HaveOccurred())
				})

				It("fail on missing values", func() {
					formValues.Del("group")
					failures := InterceptGomegaFailures(func() {
						resp, err = http.Get(s.URL() + "/foo?" + formValues.Encode())
					})
					Expect(failures).Should(HaveLen(1))
				})

				It("fail on incorrect values", func() {
					formValues.Set("group", "wheel")
					failures := InterceptGomegaFailures(func() {
						resp, err = http.Get(s.URL() + "/foo?" + formValues.Encode())
					})
					Expect(failures).Should(HaveLen(1))
				})
			})

			Context("when present in the body", func() {
				BeforeEach(func() {
					s.AppendHandlers(CombineHandlers(
						VerifyRequest("POST", "/foo"),
						VerifyForm(url.Values{
							"users": []string{"user1", "user2"},
							"group": []string{"users"},
						}),
					))
				})

				It("should verify form values", func() {
					resp, err = http.PostForm(s.URL()+"/foo", formValues)
					Expect(err).ShouldNot(HaveOccurred())
				})

				It("should ignore extra values", func() {
					formValues.Add("extra", "value")
					resp, err = http.PostForm(s.URL()+"/foo", formValues)
					Expect(err).ShouldNot(HaveOccurred())
				})

				It("fail on missing values", func() {
					formValues.Del("group")
					failures := InterceptGomegaFailures(func() {
						resp, err = http.PostForm(s.URL()+"/foo", formValues)
					})
					Expect(failures).Should(HaveLen(1))
				})

				It("fail on incorrect values", func() {
					formValues.Set("group", "wheel")
					failures := InterceptGomegaFailures(func() {
						resp, err = http.PostForm(s.URL()+"/foo", formValues)
					})
					Expect(failures).Should(HaveLen(1))
				})
			})
		})

		Describe("VerifyFormKV", func() {
			Context("when encoded in the URL", func() {
				BeforeEach(func() {
					s.AppendHandlers(CombineHandlers(
						VerifyRequest("GET", "/foo"),
						VerifyFormKV("users", "user1", "user2"),
					))
				})

				It("verifies the form value", func() {
					resp, err = http.Get(s.URL() + "/foo?users=user1&users=user2")
					Expect(err).ShouldNot(HaveOccurred())
				})

				It("verifies the form value", func() {
					failures := InterceptGomegaFailures(func() {
						resp, err = http.Get(s.URL() + "/foo?users=user1")
					})
					Expect(failures).Should(HaveLen(1))
				})
			})

			Context("when present in the body", func() {
				BeforeEach(func() {
					s.AppendHandlers(CombineHandlers(
						VerifyRequest("POST", "/foo"),
						VerifyFormKV("users", "user1", "user2"),
					))
				})

				It("verifies the form value", func() {
					resp, err = http.PostForm(s.URL()+"/foo", url.Values{"users": []string{"user1", "user2"}})
					Expect(err).ShouldNot(HaveOccurred())
				})

				It("verifies the form value", func() {
					failures := InterceptGomegaFailures(func() {
						resp, err = http.PostForm(s.URL()+"/foo", url.Values{"users": []string{"user1"}})
					})
					Expect(failures).Should(HaveLen(1))
				})
			})
		})

		Describe("VerifyProtoRepresenting", func() {
			var message *protobuf.SimpleMessage

			BeforeEach(func() {
				message = new(protobuf.SimpleMessage)
				message.Description = proto.String("A description")
				message.Id = proto.Int32(0)

				s.AppendHandlers(CombineHandlers(
					VerifyRequest("POST", "/proto"),
					VerifyProtoRepresenting(message),
				))
			})

			It("verifies the proto body and the content type", func() {
				serialized, err := proto.Marshal(message)
				Expect(err).ShouldNot(HaveOccurred())

				resp, err = http.Post(s.URL()+"/proto", "application/x-protobuf", bytes.NewReader(serialized))
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("should verify the proto body and the content type", func() {
				serialized, err := proto.Marshal(&protobuf.SimpleMessage{
					Description: proto.String("A description"),
					Id:          proto.Int32(0),
					Metadata:    proto.String("some metadata"),
				})
				Expect(err).ShouldNot(HaveOccurred())

				failures := InterceptGomegaFailures(func() {
					http.Post(s.URL()+"/proto", "application/x-protobuf", bytes.NewReader(serialized))
				})
				Expect(failures).Should(HaveLen(1))
			})

			It("should verify the proto body and the content type", func() {
				serialized, err := proto.Marshal(message)
				Expect(err).ShouldNot(HaveOccurred())

				failures := InterceptGomegaFailures(func() {
					http.Post(s.URL()+"/proto", "application/not-x-protobuf", bytes.NewReader(serialized))
				})
				Expect(failures).Should(HaveLen(1))
			})
		})

		Describe("RespondWith", func() {
			Context("without headers", func() {
				BeforeEach(func() {
					s.AppendHandlers(CombineHandlers(
						VerifyRequest("POST", "/foo"),
						RespondWith(http.StatusCreated, "sweet"),
					), CombineHandlers(
						VerifyRequest("POST", "/foo"),
						RespondWith(http.StatusOK, []byte("sour")),
					))
				})

				It("should return the response", func() {
					resp, err = http.Post(s.URL()+"/foo", "application/json", nil)
					Expect(err).ShouldNot(HaveOccurred())

					Expect(resp.StatusCode).Should(Equal(http.StatusCreated))

					body, err := ioutil.ReadAll(resp.Body)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(body).Should(Equal([]byte("sweet")))

					resp, err = http.Post(s.URL()+"/foo", "application/json", nil)
					Expect(err).ShouldNot(HaveOccurred())

					Expect(resp.StatusCode).Should(Equal(http.StatusOK))

					body, err = ioutil.ReadAll(resp.Body)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(body).Should(Equal([]byte("sour")))
				})
			})

			Context("with headers", func() {
				BeforeEach(func() {
					s.AppendHandlers(CombineHandlers(
						VerifyRequest("POST", "/foo"),
						RespondWith(http.StatusCreated, "sweet", http.Header{"X-Custom-Header": []string{"my header"}}),
					))
				})

				It("should return the headers too", func() {
					resp, err = http.Post(s.URL()+"/foo", "application/json", nil)
					Expect(err).ShouldNot(HaveOccurred())

					Expect(resp.StatusCode).Should(Equal(http.StatusCreated))
					Expect(ioutil.ReadAll(resp.Body)).Should(Equal([]byte("sweet")))
					Expect(resp.Header.Get("X-Custom-Header")).Should(Equal("my header"))
				})
			})
		})

		Describe("RespondWithPtr", func() {
			var code int
			var byteBody []byte
			var stringBody string
			BeforeEach(func() {
				code = http.StatusOK
				byteBody = []byte("sweet")
				stringBody = "sour"

				s.AppendHandlers(CombineHandlers(
					VerifyRequest("POST", "/foo"),
					RespondWithPtr(&code, &byteBody),
				), CombineHandlers(
					VerifyRequest("POST", "/foo"),
					RespondWithPtr(&code, &stringBody),
				))
			})

			It("should return the response", func() {
				code = http.StatusCreated
				byteBody = []byte("tasty")
				stringBody = "treat"

				resp, err = http.Post(s.URL()+"/foo", "application/json", nil)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(resp.StatusCode).Should(Equal(http.StatusCreated))

				body, err := ioutil.ReadAll(resp.Body)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(body).Should(Equal([]byte("tasty")))

				resp, err = http.Post(s.URL()+"/foo", "application/json", nil)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(resp.StatusCode).Should(Equal(http.StatusCreated))

				body, err = ioutil.ReadAll(resp.Body)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(body).Should(Equal([]byte("treat")))
			})

			Context("when passed a nil body", func() {
				BeforeEach(func() {
					s.SetHandler(0, CombineHandlers(
						VerifyRequest("POST", "/foo"),
						RespondWithPtr(&code, nil),
					))
				})

				It("should return an empty body and not explode", func() {
					resp, err = http.Post(s.URL()+"/foo", "application/json", nil)

					Expect(err).ShouldNot(HaveOccurred())
					Expect(resp.StatusCode).Should(Equal(http.StatusOK))
					body, err := ioutil.ReadAll(resp.Body)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(body).Should(BeEmpty())

					Expect(s.ReceivedRequests()).Should(HaveLen(1))
				})
			})
		})

		Describe("RespondWithJSON", func() {
			Context("when no optional headers are set", func() {
				BeforeEach(func() {
					s.AppendHandlers(CombineHandlers(
						VerifyRequest("POST", "/foo"),
						RespondWithJSONEncoded(http.StatusCreated, []int{1, 2, 3}),
					))
				})

				It("should return the response", func() {
					resp, err = http.Post(s.URL()+"/foo", "application/json", nil)
					Expect(err).ShouldNot(HaveOccurred())

					Expect(resp.StatusCode).Should(Equal(http.StatusCreated))

					body, err := ioutil.ReadAll(resp.Body)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(body).Should(MatchJSON("[1,2,3]"))
				})

				It("should set the Content-Type header to application/json", func() {
					resp, err = http.Post(s.URL()+"/foo", "application/json", nil)
					Expect(err).ShouldNot(HaveOccurred())

					Expect(resp.Header["Content-Type"]).Should(Equal([]string{"application/json"}))
				})
			})

			Context("when optional headers are set", func() {
				var headers http.Header
				BeforeEach(func() {
					headers = http.Header{"Stuff": []string{"things"}}
				})

				JustBeforeEach(func() {
					s.AppendHandlers(CombineHandlers(
						VerifyRequest("POST", "/foo"),
						RespondWithJSONEncoded(http.StatusCreated, []int{1, 2, 3}, headers),
					))
				})

				It("should preserve those headers", func() {
					resp, err = http.Post(s.URL()+"/foo", "application/json", nil)
					Expect(err).ShouldNot(HaveOccurred())

					Expect(resp.Header["Stuff"]).Should(Equal([]string{"things"}))
				})

				It("should set the Content-Type header to application/json", func() {
					resp, err = http.Post(s.URL()+"/foo", "application/json", nil)
					Expect(err).ShouldNot(HaveOccurred())

					Expect(resp.Header["Content-Type"]).Should(Equal([]string{"application/json"}))
				})

				Context("when setting the Content-Type explicitly", func() {
					BeforeEach(func() {
						headers["Content-Type"] = []string{"not-json"}
					})

					It("should use the Content-Type header that was explicitly set", func() {
						resp, err = http.Post(s.URL()+"/foo", "application/json", nil)
						Expect(err).ShouldNot(HaveOccurred())

						Expect(resp.Header["Content-Type"]).Should(Equal([]string{"not-json"}))
					})
				})
			})
		})

		Describe("RespondWithJSONPtr", func() {
			type testObject struct {
				Key   string
				Value string
			}

			var code int
			var object testObject

			Context("when no optional headers are set", func() {
				BeforeEach(func() {
					code = http.StatusOK
					object = testObject{}
					s.AppendHandlers(CombineHandlers(
						VerifyRequest("POST", "/foo"),
						RespondWithJSONEncodedPtr(&code, &object),
					))
				})

				It("should return the response", func() {
					code = http.StatusCreated
					object = testObject{
						Key:   "Jim",
						Value: "Codes",
					}
					resp, err = http.Post(s.URL()+"/foo", "application/json", nil)
					Expect(err).ShouldNot(HaveOccurred())

					Expect(resp.StatusCode).Should(Equal(http.StatusCreated))

					body, err := ioutil.ReadAll(resp.Body)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(body).Should(MatchJSON(`{"Key": "Jim", "Value": "Codes"}`))
				})

				It("should set the Content-Type header to application/json", func() {
					resp, err = http.Post(s.URL()+"/foo", "application/json", nil)
					Expect(err).ShouldNot(HaveOccurred())

					Expect(resp.Header["Content-Type"]).Should(Equal([]string{"application/json"}))
				})
			})

			Context("when optional headers are set", func() {
				var headers http.Header
				BeforeEach(func() {
					headers = http.Header{"Stuff": []string{"things"}}
				})

				JustBeforeEach(func() {
					code = http.StatusOK
					object = testObject{}
					s.AppendHandlers(CombineHandlers(
						VerifyRequest("POST", "/foo"),
						RespondWithJSONEncodedPtr(&code, &object, headers),
					))
				})

				It("should preserve those headers", func() {
					resp, err = http.Post(s.URL()+"/foo", "application/json", nil)
					Expect(err).ShouldNot(HaveOccurred())

					Expect(resp.Header["Stuff"]).Should(Equal([]string{"things"}))
				})

				It("should set the Content-Type header to application/json", func() {
					resp, err = http.Post(s.URL()+"/foo", "application/json", nil)
					Expect(err).ShouldNot(HaveOccurred())

					Expect(resp.Header["Content-Type"]).Should(Equal([]string{"application/json"}))
				})

				Context("when setting the Content-Type explicitly", func() {
					BeforeEach(func() {
						headers["Content-Type"] = []string{"not-json"}
					})

					It("should use the Content-Type header that was explicitly set", func() {
						resp, err = http.Post(s.URL()+"/foo", "application/json", nil)
						Expect(err).ShouldNot(HaveOccurred())

						Expect(resp.Header["Content-Type"]).Should(Equal([]string{"not-json"}))
					})
				})
			})
		})

		Describe("RespondWithProto", func() {
			var message *protobuf.SimpleMessage

			BeforeEach(func() {
				message = new(protobuf.SimpleMessage)
				message.Description = proto.String("A description")
				message.Id = proto.Int32(99)
			})

			Context("when no optional headers are set", func() {
				BeforeEach(func() {
					s.AppendHandlers(CombineHandlers(
						VerifyRequest("POST", "/proto"),
						RespondWithProto(http.StatusCreated, message),
					))
				})

				It("should return the response", func() {
					resp, err = http.Post(s.URL()+"/proto", "application/x-protobuf", nil)
					Expect(err).ShouldNot(HaveOccurred())

					Expect(resp.StatusCode).Should(Equal(http.StatusCreated))

					var received protobuf.SimpleMessage
					body, err := ioutil.ReadAll(resp.Body)
					err = proto.Unmarshal(body, &received)
					Expect(err).ShouldNot(HaveOccurred())
				})

				It("should set the Content-Type header to application/x-protobuf", func() {
					resp, err = http.Post(s.URL()+"/proto", "application/x-protobuf", nil)
					Expect(err).ShouldNot(HaveOccurred())

					Expect(resp.Header["Content-Type"]).Should(Equal([]string{"application/x-protobuf"}))
				})
			})

			Context("when optional headers are set", func() {
				var headers http.Header
				BeforeEach(func() {
					headers = http.Header{"Stuff": []string{"things"}}
				})

				JustBeforeEach(func() {
					s.AppendHandlers(CombineHandlers(
						VerifyRequest("POST", "/proto"),
						RespondWithProto(http.StatusCreated, message, headers),
					))
				})

				It("should preserve those headers", func() {
					resp, err = http.Post(s.URL()+"/proto", "application/x-protobuf", nil)
					Expect(err).ShouldNot(HaveOccurred())

					Expect(resp.Header["Stuff"]).Should(Equal([]string{"things"}))
				})

				It("should set the Content-Type header to application/x-protobuf", func() {
					resp, err = http.Post(s.URL()+"/proto", "application/x-protobuf", nil)
					Expect(err).ShouldNot(HaveOccurred())

					Expect(resp.Header["Content-Type"]).Should(Equal([]string{"application/x-protobuf"}))
				})

				Context("when setting the Content-Type explicitly", func() {
					BeforeEach(func() {
						headers["Content-Type"] = []string{"not-x-protobuf"}
					})

					It("should use the Content-Type header that was explicitly set", func() {
						resp, err = http.Post(s.URL()+"/proto", "application/x-protobuf", nil)
						Expect(err).ShouldNot(HaveOccurred())

						Expect(resp.Header["Content-Type"]).Should(Equal([]string{"not-x-protobuf"}))
					})
				})
			})
		})
	})
})
