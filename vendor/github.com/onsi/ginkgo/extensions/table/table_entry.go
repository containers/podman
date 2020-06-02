package table

import (
	"reflect"

	"github.com/onsi/ginkgo/internal/codelocation"
	"github.com/onsi/ginkgo/internal/global"
	"github.com/onsi/ginkgo/types"
)

/*
TableEntry represents an entry in a table test.  You generally use the `Entry` constructor.
*/
type TableEntry struct {
	Description  string
	Parameters   []interface{}
	Pending      bool
	Focused      bool
	codeLocation types.CodeLocation
}

func (t TableEntry) generateIt(itBody reflect.Value) {
	if t.codeLocation == (types.CodeLocation{}) {
		// The user created the TableEntry struct directly instead of having used the (F/P/X)Entry constructors.
		// Therefore default to the code location of the surrounding DescribeTable.
		t.codeLocation = codelocation.New(5)
	}

	if t.Pending {
		global.Suite.PushItNode(t.Description, func() {}, types.FlagTypePending, t.codeLocation, 0)
		return
	}

	values := make([]reflect.Value, len(t.Parameters))
	iBodyType := itBody.Type()
	for i, param := range t.Parameters {
		if param == nil {
			inType := iBodyType.In(i)
			values[i] = reflect.Zero(inType)
		} else {
			values[i] = reflect.ValueOf(param)
		}
	}

	body := func() {
		itBody.Call(values)
	}

	if t.Focused {
		global.Suite.PushItNode(t.Description, body, types.FlagTypeFocused, t.codeLocation, global.DefaultTimeout)
	} else {
		global.Suite.PushItNode(t.Description, body, types.FlagTypeNone, t.codeLocation, global.DefaultTimeout)
	}
}

/*
Entry constructs a TableEntry.

The first argument is a required description (this becomes the content of the generated Ginkgo `It`).
Subsequent parameters are saved off and sent to the callback passed in to `DescribeTable`.

Each Entry ends up generating an individual Ginkgo It.
*/
func Entry(description string, parameters ...interface{}) TableEntry {
	return TableEntry{description, parameters, false, false, codelocation.New(1)}
}

/*
You can focus a particular entry with FEntry.  This is equivalent to FIt.
*/
func FEntry(description string, parameters ...interface{}) TableEntry {
	return TableEntry{description, parameters, false, true, codelocation.New(1)}
}

/*
You can mark a particular entry as pending with PEntry.  This is equivalent to PIt.
*/
func PEntry(description string, parameters ...interface{}) TableEntry {
	return TableEntry{description, parameters, true, false, codelocation.New(1)}
}

/*
You can mark a particular entry as pending with XEntry.  This is equivalent to XIt.
*/
func XEntry(description string, parameters ...interface{}) TableEntry {
	return TableEntry{description, parameters, true, false, codelocation.New(1)}
}
