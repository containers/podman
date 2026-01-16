//go:build windows

package hypervctl

import (
	"encoding/json"
	"strings"
)

// StringOrArray represents a field that can be either a string or an array of strings
type StringOrArray []string

// UnmarshalJSON implements custom unmarshaling for string or array of strings
func (s *StringOrArray) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as an array first
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*s = StringOrArray(arr)
		return nil
	}

	// If that fails, try to unmarshal as a single string
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	// Wrap the single string in an array
	*s = StringOrArray([]string{str})
	return nil
}

// String returns a comma-separated string of all values
func (s StringOrArray) String() string {
	return strings.Join([]string(s), ", ")
}

// First returns the first element or empty string if empty
func (s StringOrArray) First() string {
	if len(s) > 0 {
		return s[0]
	}
	return ""
}
