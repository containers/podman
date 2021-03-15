// Copyright 2020 The go-libvirt Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package event

import "context"

// Stream is an unbounded buffered event channel. The implementation
// consists of a pair of unbuffered channels and a goroutine to manage them.
// Client behavior will not cause incoming events to block.
type Stream struct {
	// Program specifies the source of the events - libvirt or QEMU.
	Program uint32

	// CallbackID is returned by the event registration call.
	CallbackID int32

	// manage unbounded channel behavior.
	queue   []Event
	in, out chan Event

	// terminates processing
	shutdown context.CancelFunc
}

// Recv returns the next available event from the Stream's queue.
func (s *Stream) Recv() chan Event {
	return s.out
}

// Push appends a new event to the queue.
func (s *Stream) Push(e Event) {
	s.in <- e
}

// Shutdown gracefully terminates Stream processing, releasing all
// internal resources. Events which have not yet been received by the client
// will be dropped. Subsequent calls to Shutdown() are idempotent.
func (s *Stream) Shutdown() {
	if s.shutdown != nil {
		s.shutdown()
	}
}

// start starts the event processing loop, which will continue to run until
// terminated by the returned context.CancelFunc. Starting a previously started
// Stream is an idempotent operation.
func (s *Stream) start() context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())

	go s.process(ctx)

	return cancel
}

// process manages an Stream's lifecycle until canceled by the provided
// context.  Incoming events are appended to a queue which is then relayed to
// the a listening client. New events pushed onto the queue will not block due
// to client behavior.
func (s *Stream) process(ctx context.Context) {
	defer func() {
		close(s.in)
		close(s.out)
	}()

	for {
		// informs send() to stop trying
		nctx, next := context.WithCancel(ctx)
		defer next()

		select {
		// new event received, append to queue
		case e := <-s.in:
			s.queue = append(s.queue, e)

		// client recieved an event, pop from queue
		case <-s.send(nctx):
			if len(s.queue) > 1 {
				s.queue = s.queue[1:]
			} else {
				s.queue = []Event{}
			}

		// shutdown requested
		case <-ctx.Done():
			return

		}

		next()
	}
}

// send returns a channel which blocks until either the first item on the queue
// (if existing) is sent to the client, or the provided context is canceled.
// The stream's queue is never modified.
func (s *Stream) send(ctx context.Context) <-chan struct{} {
	ch := make(chan struct{})

	go func() {
		defer close(ch)

		// do nothing and block if the queue is empty
		if len(s.queue) == 0 {
			<-ctx.Done()
			return
		}

		// otherwise, attempt to send the event
		select {
		case s.out <- s.queue[0]:
		case <-ctx.Done():
		}
	}()

	return ch
}

// NewStream configures a new Event Stream. Incoming events are appended to a
// queue, which is then relayed to the listening client. Client behavior will
// not cause incoming events to block. It is the responsibility of the caller
// to terminate the Stream via Shutdown() when no longer in use.
func NewStream(program uint32, cbID int32) *Stream {
	ic := &Stream{
		Program:    program,
		CallbackID: cbID,
		in:         make(chan Event),
		out:        make(chan Event),
	}

	ic.shutdown = ic.start()

	return ic
}
