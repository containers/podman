package cmdline

import (
	"fmt"
	"strings"

	"github.com/crc-org/vfkit/pkg/util"
)

// -- stringSlice Value
type stringSliceValue struct {
	value   []string
	changed bool
}

type strvBuilder struct {
	strBuilder strings.Builder
	strv       []string
	err        error
}

func (builder *strvBuilder) String() string {
	str := builder.strBuilder.String()
	// "a","b" is parsed as { "a", "b" } to match pflag StringSlice behaviour
	return util.TrimQuotes(str)
}

func (builder *strvBuilder) Next() {
	str := builder.String()
	builder.strv = append(builder.strv, str)
	builder.strBuilder.Reset()
}

func (builder *strvBuilder) WriteRune(r rune) {
	if builder.err != nil {
		return
	}
	_, err := builder.strBuilder.WriteRune(r)
	if err != nil {
		builder.err = err
	}
}

func (builder *strvBuilder) End() ([]string, error) {
	if builder.err != nil {
		return nil, builder.err
	}

	lastStr := builder.String()
	if len(builder.strv) != 0 || len(lastStr) != 0 {
		builder.strv = append(builder.strv, lastStr)
	}

	return builder.strv, nil
}

func parseString(str string) ([]string, error) {
	withinQuotes := false

	//  trim spaces from str
	builder := strvBuilder{}
	for _, c := range str {
		if withinQuotes {
			if c == '"' {
				withinQuotes = false
			}
			builder.WriteRune(c)
			continue
		}
		if withinQuotes {
			return nil, fmt.Errorf("Coding error in arg parsing")
		}
		switch c {
		case ',':
			builder.Next()
		case '"':
			withinQuotes = true
			fallthrough
		// FIXME: can we get ' ' at this point??
		default:
			builder.WriteRune(c)
		}
	}
	if withinQuotes {
		return nil, fmt.Errorf("Mismatched \"")
	}

	return builder.End()
}

func (s *stringSliceValue) Set(val string) error {
	v, err := parseString(val)
	if err != nil {
		return err
	}
	if !s.changed {
		s.value = v
	} else {
		s.value = append(s.value, v...)
	}
	s.changed = true
	return nil
}

func (s *stringSliceValue) Type() string {
	return "stringSlice"
}

func (s *stringSliceValue) String() string {
	if s == nil || s.value == nil {
		return "[]"
	}
	return "[" + strings.Join(s.value, ",") + "]"
}

func (s *stringSliceValue) Append(val string) error {
	s.value = append(s.value, val)
	return nil
}

func (s *stringSliceValue) Replace(val []string) error {
	s.value = val
	return nil
}

func (s *stringSliceValue) GetSlice() []string {
	return s.value
}
