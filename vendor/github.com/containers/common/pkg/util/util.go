package util

import "regexp"

// StringInSlice determines if a string is in a string slice, returns bool
func StringInSlice(s string, sl []string) bool {
	for _, i := range sl {
		if i == s {
			return true
		}
	}
	return false
}

// StringMatchRegexSlice determines if a given string matches one of the given regexes, returns bool
func StringMatchRegexSlice(s string, re []string) bool {
	for _, r := range re {
		m, err := regexp.MatchString(r, s)
		if err == nil && m {
			return true
		}
	}
	return false
}
