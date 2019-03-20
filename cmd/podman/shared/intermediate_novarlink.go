// +build !varlink
// +build !remoteclient

package shared

/*
attention

in this file you will see alot of struct duplication.  this was done because people wanted a strongly typed
varlink mechanism.  this resulted in us creating this intermediate layer that allows us to take the input
from the cli and make an intermediate layer which can be transferred as strongly typed structures over a varlink
interface.

we intentionally avoided heavy use of reflection here because we were concerned about performance impacts to the
non-varlink intermediate layer generation.
*/

// ToString wrapper for build without varlink
func (c CRStringSlice) ToVarlink() interface{} {
	var v interface{}
	return v
}

// ToString wrapper for build without varlink
func (c CRString) ToVarlink() interface{} {
	var v interface{}
	return v
}

// ToString wrapper for build without varlink
func (c CRBool) ToVarlink() interface{} {
	var v interface{}
	return v
}

// ToString wrapper for build without varlink
func (c CRUint64) ToVarlink() interface{} {
	var v interface{}
	return v
}

// ToString wrapper for build without varlink
func (c CRInt64) ToVarlink() interface{} {
	var v interface{}
	return v
}

// ToString wrapper for build without varlink
func (c CRFloat64) ToVarlink() interface{} {
	var v interface{}
	return v
}

// ToString wrapper for build without varlink
func (c CRUint) ToVarlink() interface{} {
	var v interface{}
	return v
}

// ToString wrapper for build without varlink
func (c CRStringArray) ToVarlink() interface{} {
	var v interface{}
	return v
}

// ToString wrapper for build without varlink
func (c CRInt) ToVarlink() interface{} {
	var v interface{}
	return v
}
