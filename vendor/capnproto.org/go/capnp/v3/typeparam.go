package capnp

// The TypeParam interface must be satisified by a type to be used as a capnproto
// type parameter. This is satisified by all capnproto pointer types. T should
// be instantiated by the type itself; in type parameter lists you will typically
// see parameter/constraints like T TypeParam[T].
type TypeParam[T any] interface {
	// Convert the receiver to a Ptr, storing it in seg if it is not
	// already associated with some message (only true for Clients and
	// wrappers around them.
	EncodeAsPtr(seg *Segment) Ptr

	// Decode the pointer as the type of the receiver. Generally,
	// the reciever will be the zero value for the type.
	DecodeFromPtr(p Ptr) T
}
