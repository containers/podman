package varlinkapi

import (
	"github.com/sirupsen/logrus"
)

/*
attention

in this file you will see a lot of struct duplication.  this was done because people wanted a strongly typed
varlink mechanism.  this resulted in us creating this intermediate layer that allows us to take the input
from the cli and make an intermediate layer which can be transferred as strongly typed structures over a varlink
interface.

we intentionally avoided heavy use of reflection here because we were concerned about performance impacts to the
non-varlink intermediate layer generation.
*/

// GenericCLIResult describes the overall interface for dealing with
// the create command cli in both local and remote uses
type GenericCLIResult interface {
	IsSet() bool
	Name() string
	Value() interface{}
}

// CRStringSlice describes a string slice cli struct
type CRStringSlice struct {
	Val []string
	createResult
}

// CRString describes a string cli struct
type CRString struct {
	Val string
	createResult
}

// CRUint64 describes a uint64 cli struct
type CRUint64 struct {
	Val uint64
	createResult
}

// CRFloat64 describes a float64 cli struct
type CRFloat64 struct {
	Val float64
	createResult
}

//CRBool describes a bool cli struct
type CRBool struct {
	Val bool
	createResult
}

// CRInt64 describes an int64 cli struct
type CRInt64 struct {
	Val int64
	createResult
}

// CRUint describes a uint cli struct
type CRUint struct {
	Val uint
	createResult
}

// CRInt describes an int cli struct
type CRInt struct {
	Val int
	createResult
}

// CRStringArray describes a stringarray cli struct
type CRStringArray struct {
	Val []string
	createResult
}

type createResult struct {
	Flag    string
	Changed bool
}

// GenericCLIResults in the intermediate object between the cobra cli
// and createconfig
type GenericCLIResults struct {
	results   map[string]GenericCLIResult
	InputArgs []string
}

// IsSet returns a bool if the flag was changed
func (f GenericCLIResults) IsSet(flag string) bool {
	r := f.findResult(flag)
	if r == nil {
		return false
	}
	return r.IsSet()
}

// Value returns the value of the cli flag
func (f GenericCLIResults) Value(flag string) interface{} {
	r := f.findResult(flag)
	if r == nil {
		return ""
	}
	return r.Value()
}

func (f GenericCLIResults) findResult(flag string) GenericCLIResult {
	val, ok := f.results[flag]
	if ok {
		return val
	}
	logrus.Debugf("unable to find flag %s", flag)
	return nil
}

// Bool is a wrapper to get a bool value from GenericCLIResults
func (f GenericCLIResults) Bool(flag string) bool {
	r := f.findResult(flag)
	if r == nil {
		return false
	}
	return r.Value().(bool)
}

// String is a wrapper to get a string value from GenericCLIResults
func (f GenericCLIResults) String(flag string) string {
	r := f.findResult(flag)
	if r == nil {
		return ""
	}
	return r.Value().(string)
}

// Uint is a wrapper to get an uint value from GenericCLIResults
func (f GenericCLIResults) Uint(flag string) uint {
	r := f.findResult(flag)
	if r == nil {
		return 0
	}
	return r.Value().(uint)
}

// StringSlice is a wrapper to get a stringslice value from GenericCLIResults
func (f GenericCLIResults) StringSlice(flag string) []string {
	r := f.findResult(flag)
	if r == nil {
		return []string{}
	}
	return r.Value().([]string)
}

// StringArray is a wrapper to get a stringslice value from GenericCLIResults
func (f GenericCLIResults) StringArray(flag string) []string {
	r := f.findResult(flag)
	if r == nil {
		return []string{}
	}
	return r.Value().([]string)
}

// Uint64 is a wrapper to get an uint64 value from GenericCLIResults
func (f GenericCLIResults) Uint64(flag string) uint64 {
	r := f.findResult(flag)
	if r == nil {
		return 0
	}
	return r.Value().(uint64)
}

// Int64 is a wrapper to get an int64 value from GenericCLIResults
func (f GenericCLIResults) Int64(flag string) int64 {
	r := f.findResult(flag)
	if r == nil {
		return 0
	}
	return r.Value().(int64)
}

// Int is a wrapper to get an int value from GenericCLIResults
func (f GenericCLIResults) Int(flag string) int {
	r := f.findResult(flag)
	if r == nil {
		return 0
	}
	return r.Value().(int)
}

// Float64 is a wrapper to get an float64 value from GenericCLIResults
func (f GenericCLIResults) Float64(flag string) float64 {
	r := f.findResult(flag)
	if r == nil {
		return 0
	}
	return r.Value().(float64)
}

// Float64 is a wrapper to get an float64 value from GenericCLIResults
func (f GenericCLIResults) Changed(flag string) bool {
	r := f.findResult(flag)
	if r == nil {
		return false
	}
	return r.IsSet()
}

// IsSet ...
func (c CRStringSlice) IsSet() bool { return c.Changed }

// Name ...
func (c CRStringSlice) Name() string { return c.Flag }

// Value ...
func (c CRStringSlice) Value() interface{} { return c.Val }

// IsSet ...
func (c CRString) IsSet() bool { return c.Changed }

// Name ...
func (c CRString) Name() string { return c.Flag }

// Value ...
func (c CRString) Value() interface{} { return c.Val }

// IsSet ...
func (c CRUint64) IsSet() bool { return c.Changed }

// Name ...
func (c CRUint64) Name() string { return c.Flag }

// Value ...
func (c CRUint64) Value() interface{} { return c.Val }

// IsSet ...
func (c CRFloat64) IsSet() bool { return c.Changed }

// Name ...
func (c CRFloat64) Name() string { return c.Flag }

// Value ...
func (c CRFloat64) Value() interface{} { return c.Val }

// IsSet ...
func (c CRBool) IsSet() bool { return c.Changed }

// Name ...
func (c CRBool) Name() string { return c.Flag }

// Value ...
func (c CRBool) Value() interface{} { return c.Val }

// IsSet ...
func (c CRInt64) IsSet() bool { return c.Changed }

// Name ...
func (c CRInt64) Name() string { return c.Flag }

// Value ...
func (c CRInt64) Value() interface{} { return c.Val }

// IsSet ...
func (c CRUint) IsSet() bool { return c.Changed }

// Name ...
func (c CRUint) Name() string { return c.Flag }

// Value ...
func (c CRUint) Value() interface{} { return c.Val }

// IsSet ...
func (c CRInt) IsSet() bool { return c.Changed }

// Name ...
func (c CRInt) Name() string { return c.Flag }

// Value ...
func (c CRInt) Value() interface{} { return c.Val }

// IsSet ...
func (c CRStringArray) IsSet() bool { return c.Changed }

// Name ...
func (c CRStringArray) Name() string { return c.Flag }

// Value ...
func (c CRStringArray) Value() interface{} { return c.Val }
