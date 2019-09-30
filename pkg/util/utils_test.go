package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	sliceData = []string{"one", "two", "three", "four"}
)

func TestStringInSlice(t *testing.T) {
	// string is in the slice
	assert.True(t, StringInSlice("one", sliceData))
	// string is not in the slice
	assert.False(t, StringInSlice("five", sliceData))
	// string is not in empty slice
	assert.False(t, StringInSlice("one", []string{}))
}

func TestParseChanges(t *testing.T) {
	// CMD=/bin/sh
	_, vals, err := ParseChanges("CMD=/bin/sh")
	assert.EqualValues(t, []string{"/bin/sh"}, vals)
	assert.NoError(t, err)

	// CMD [/bin/sh]
	_, vals, err = ParseChanges("CMD [/bin/sh]")
	assert.EqualValues(t, []string{"/bin/sh"}, vals)
	assert.NoError(t, err)

	// CMD ["/bin/sh"]
	_, vals, err = ParseChanges(`CMD ["/bin/sh"]`)
	assert.EqualValues(t, []string{`"/bin/sh"`}, vals)
	assert.NoError(t, err)

	// CMD ["/bin/sh","-c","ls"]
	_, vals, err = ParseChanges(`CMD ["/bin/sh","c","ls"]`)
	assert.EqualValues(t, []string{`"/bin/sh"`, `"c"`, `"ls"`}, vals)
	assert.NoError(t, err)

	// CMD ["/bin/sh","arg-with,comma"]
	_, vals, err = ParseChanges(`CMD ["/bin/sh","arg-with,comma"]`)
	assert.EqualValues(t, []string{`"/bin/sh"`, `"arg-with`, `comma"`}, vals)
	assert.NoError(t, err)

	// CMD "/bin/sh"]
	_, _, err = ParseChanges(`CMD "/bin/sh"]`)
	assert.Error(t, err)
	assert.Equal(t, `invalid value "/bin/sh"]`, err.Error())

	// CMD [bin/sh
	_, _, err = ParseChanges(`CMD "/bin/sh"]`)
	assert.Error(t, err)
	assert.Equal(t, `invalid value "/bin/sh"]`, err.Error())

	// CMD ["/bin /sh"]
	_, _, err = ParseChanges(`CMD ["/bin /sh"]`)
	assert.Error(t, err)
	assert.Equal(t, `invalid value "/bin /sh"`, err.Error())

	// CMD ["/bin/sh", "-c","ls"] whitespace between values
	_, vals, err = ParseChanges(`CMD ["/bin/sh", "c","ls"]`)
	assert.Error(t, err)
	assert.Equal(t, `invalid value  "c"`, err.Error())

	// CMD?
	_, _, err = ParseChanges(`CMD?`)
	assert.Error(t, err)
	assert.Equal(t, `invalid format CMD?`, err.Error())

	// empty values for CMD
	_, _, err = ParseChanges(`CMD `)
	assert.Error(t, err)
	assert.Equal(t, `invalid value `, err.Error())

	// LABEL=blue=image
	_, vals, err = ParseChanges(`LABEL=blue=image`)
	assert.EqualValues(t, []string{"blue", "image"}, vals)
	assert.NoError(t, err)

	// LABEL = blue=image
	_, vals, err = ParseChanges(`LABEL = blue=image`)
	assert.Error(t, err)
	assert.Equal(t, `invalid value = blue=image`, err.Error())

}
