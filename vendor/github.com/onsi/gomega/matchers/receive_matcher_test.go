package matchers_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/matchers"
)

type kungFuActor interface {
	DrunkenMaster() bool
}

type jackie struct {
	name string
}

func (j *jackie) DrunkenMaster() bool {
	return true
}

type someError struct{ s string }

func (e *someError) Error() string { return e.s }

var _ = Describe("ReceiveMatcher", func() {
	Context("with no argument", func() {
		Context("for a buffered channel", func() {
			It("should succeed", func() {
				channel := make(chan bool, 1)

				Expect(channel).ShouldNot(Receive())

				channel <- true

				Expect(channel).Should(Receive())
			})
		})

		Context("for an unbuffered channel", func() {
			It("should succeed (eventually)", func() {
				channel := make(chan bool)

				Expect(channel).ShouldNot(Receive())

				go func() {
					time.Sleep(10 * time.Millisecond)
					channel <- true
				}()

				Eventually(channel).Should(Receive())
			})
		})
	})

	Context("with a pointer argument", func() {
		Context("of the correct type", func() {
			Context("when the channel has an interface type", func() {
				It("should write the value received on the channel to the pointer", func() {
					channel := make(chan error, 1)

					var value *someError

					Ω(channel).ShouldNot(Receive(&value))
					Ω(value).Should(BeZero())

					channel <- &someError{"boooom!"}

					Ω(channel).Should(Receive(&value))
					Ω(value).Should(MatchError("boooom!"))
				})
			})
		})

		Context("of the correct type", func() {
			It("should write the value received on the channel to the pointer", func() {
				channel := make(chan int, 1)

				var value int

				Expect(channel).ShouldNot(Receive(&value))
				Expect(value).Should(BeZero())

				channel <- 17

				Expect(channel).Should(Receive(&value))
				Expect(value).Should(Equal(17))
			})
		})

		Context("to various types of objects", func() {
			It("should work", func() {
				//channels of strings
				stringChan := make(chan string, 1)
				stringChan <- "foo"

				var s string
				Expect(stringChan).Should(Receive(&s))
				Expect(s).Should(Equal("foo"))

				//channels of slices
				sliceChan := make(chan []bool, 1)
				sliceChan <- []bool{true, true, false}

				var sl []bool
				Expect(sliceChan).Should(Receive(&sl))
				Expect(sl).Should(Equal([]bool{true, true, false}))

				//channels of channels
				chanChan := make(chan chan bool, 1)
				c := make(chan bool)
				chanChan <- c

				var receivedC chan bool
				Expect(chanChan).Should(Receive(&receivedC))
				Expect(receivedC).Should(Equal(c))

				//channels of interfaces
				jackieChan := make(chan kungFuActor, 1)
				aJackie := &jackie{name: "Jackie Chan"}
				jackieChan <- aJackie

				var theJackie kungFuActor
				Expect(jackieChan).Should(Receive(&theJackie))
				Expect(theJackie).Should(Equal(aJackie))
			})
		})

		Context("of the wrong type", func() {
			It("should error", func() {
				channel := make(chan int, 1)
				channel <- 10

				var incorrectType bool

				success, err := (&ReceiveMatcher{Arg: &incorrectType}).Match(channel)
				Expect(success).Should(BeFalse())
				Expect(err).Should(HaveOccurred())

				var notAPointer int
				success, err = (&ReceiveMatcher{Arg: notAPointer}).Match(channel)
				Expect(success).Should(BeFalse())
				Expect(err).Should(HaveOccurred())
			})
		})
	})

	Context("with a matcher", func() {
		It("should defer to the underlying matcher", func() {
			intChannel := make(chan int, 1)
			intChannel <- 3
			Expect(intChannel).Should(Receive(Equal(3)))

			intChannel <- 2
			Expect(intChannel).ShouldNot(Receive(Equal(3)))

			stringChannel := make(chan []string, 1)
			stringChannel <- []string{"foo", "bar", "baz"}
			Expect(stringChannel).Should(Receive(ContainElement(ContainSubstring("fo"))))

			stringChannel <- []string{"foo", "bar", "baz"}
			Expect(stringChannel).ShouldNot(Receive(ContainElement(ContainSubstring("archipelago"))))
		})

		It("should defer to the underlying matcher for the message", func() {
			matcher := Receive(Equal(3))
			channel := make(chan int, 1)
			channel <- 2
			matcher.Match(channel)
			Expect(matcher.FailureMessage(channel)).Should(MatchRegexp(`Expected\s+<int>: 2\s+to equal\s+<int>: 3`))

			channel <- 3
			matcher.Match(channel)
			Expect(matcher.NegatedFailureMessage(channel)).Should(MatchRegexp(`Expected\s+<int>: 3\s+not to equal\s+<int>: 3`))
		})

		It("should work just fine with Eventually", func() {
			stringChannel := make(chan string)

			go func() {
				time.Sleep(5 * time.Millisecond)
				stringChannel <- "A"
				time.Sleep(5 * time.Millisecond)
				stringChannel <- "B"
			}()

			Eventually(stringChannel).Should(Receive(Equal("B")))
		})

		Context("if the matcher errors", func() {
			It("should error", func() {
				channel := make(chan int, 1)
				channel <- 3
				success, err := (&ReceiveMatcher{Arg: ContainSubstring("three")}).Match(channel)
				Expect(success).Should(BeFalse())
				Expect(err).Should(HaveOccurred())
			})
		})

		Context("if nothing is received", func() {
			It("should fail", func() {
				channel := make(chan int, 1)
				success, err := (&ReceiveMatcher{Arg: Equal(1)}).Match(channel)
				Expect(success).Should(BeFalse())
				Expect(err).ShouldNot(HaveOccurred())
			})
		})
	})

	Context("When actual is a *closed* channel", func() {
		Context("for a buffered channel", func() {
			It("should work until it hits the end of the buffer", func() {
				channel := make(chan bool, 1)
				channel <- true

				close(channel)

				Expect(channel).Should(Receive())
				Expect(channel).ShouldNot(Receive())
			})
		})

		Context("for an unbuffered channel", func() {
			It("should always fail", func() {
				channel := make(chan bool)
				close(channel)

				Expect(channel).ShouldNot(Receive())
			})
		})
	})

	Context("When actual is a send-only channel", func() {
		It("should error", func() {
			channel := make(chan bool)

			var writerChannel chan<- bool
			writerChannel = channel

			success, err := (&ReceiveMatcher{}).Match(writerChannel)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())
		})
	})

	Context("when acutal is a non-channel", func() {
		It("should error", func() {
			var nilChannel chan bool

			success, err := (&ReceiveMatcher{}).Match(nilChannel)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())

			success, err = (&ReceiveMatcher{}).Match(nil)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())

			success, err = (&ReceiveMatcher{}).Match(3)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())
		})
	})

	Describe("when used with eventually and a custom matcher", func() {
		It("should return the matcher's error when a failing value is received on the channel, instead of the must receive something failure", func() {
			failures := InterceptGomegaFailures(func() {
				c := make(chan string, 0)
				Eventually(c, 0.01).Should(Receive(Equal("hello")))
			})
			Expect(failures[0]).Should(ContainSubstring("When passed a matcher, ReceiveMatcher's channel *must* receive something."))

			failures = InterceptGomegaFailures(func() {
				c := make(chan string, 1)
				c <- "hi"
				Eventually(c, 0.01).Should(Receive(Equal("hello")))
			})
			Expect(failures[0]).Should(ContainSubstring("<string>: hello"))
		})
	})

	Describe("Bailing early", func() {
		It("should bail early when passed a closed channel", func() {
			c := make(chan bool)
			close(c)

			t := time.Now()
			failures := InterceptGomegaFailures(func() {
				Eventually(c).Should(Receive())
			})
			Expect(time.Since(t)).Should(BeNumerically("<", 500*time.Millisecond))
			Expect(failures).Should(HaveLen(1))
		})

		It("should bail early when passed a non-channel", func() {
			t := time.Now()
			failures := InterceptGomegaFailures(func() {
				Eventually(3).Should(Receive())
			})
			Expect(time.Since(t)).Should(BeNumerically("<", 500*time.Millisecond))
			Expect(failures).Should(HaveLen(1))
		})
	})
})
