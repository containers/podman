// Package mpsc implements a multiple-producer, single-consumer queue.
package mpsc

import (
	"capnproto.org/go/capnp/v3/internal/chanmutex"
	"context"
)

// A multiple-producer, single-consumer queue. Create one with New(),
// and send from many gorotuines with Tx.Send(). Only one gorotuine may
// call Rx.Recv().
type Queue[T any] struct {
	Tx[T]
	Rx[T]
}

// The receive end of a Queue.
type Rx[T any] struct {
	// The head of the list. If the list is empty, this will be
	// non-nil but have a locked mu field.
	head *node[T]
}

// The send/transmit end of a Queue.
type Tx[T any] struct {
	// Mutex which must be held by senders. A goroutine must hold this
	// lock to manipulate `tail`.
	mu chanmutex.Mutex

	// Pointer to the tail of the list. This will have a locked mu,
	// and zero values for other fields.
	tail *node[T]
}

// A node in the linked linst that makes up the queue internally.
type node[T any] struct {
	// A mutex which guards the other fields in the node.
	// Nodes start out with this locked, and then we unlock it
	// after filling in the other fields.
	mu chanmutex.Mutex

	// The next node in the list, if any. Must be non-nil if
	// mu is unlocked:
	next *node[T]

	// The value in this node:
	value T
}

// Create a new node, with a locked mutex and zero values for
// the other fields.
func newNode[T any]() *node[T] {
	return &node[T]{mu: chanmutex.NewLocked()}
}

// Create a new, initially empty Queue.
func New[T any]() *Queue[T] {
	node := newNode[T]()
	return &Queue[T]{
		Tx: Tx[T]{
			tail: node,
			mu:   chanmutex.NewUnlocked(),
		},
		Rx: Rx[T]{head: node},
	}
}

// Send a message on the queue.
func (tx *Tx[T]) Send(v T) {
	newTail := newNode[T]()

	tx.mu.Lock()

	oldTail := tx.tail
	oldTail.next = newTail
	oldTail.value = v
	tx.tail = newTail
	oldTail.mu.Unlock()

	tx.mu.Unlock()
}

// Receive a message from the queue. Blocks if the queue is empty.
// If the context ends before the receive happens, this returns
// ctx.Err().
func (rx *Rx[T]) Recv(ctx context.Context) (T, error) {
	var zero T
	select {
	case <-rx.head.mu:
		return rx.doRecv(), nil
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}

// Try to receive a message from the queue. If successful, ok will be true.
// If the queue is empty, this will return immediately with ok = false.
func (rx *Rx[T]) TryRecv() (v T, ok bool) {
	var zero T
	select {
	case <-rx.head.mu:
		return rx.doRecv(), true
	default:
		return zero, false
	}
}

// Helper for shared logic between Recv and TryRecv. Must be holding
// rx.head.mu.
func (rx *Rx[T]) doRecv() T {
	ret := rx.head.value
	rx.head = rx.head.next
	return ret
}
