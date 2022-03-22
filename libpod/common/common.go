package common

// IsTrue determines whether the given string equals "true".
func IsTrue(str string) bool {
	return str == "true"
}

// IsFalse determines whether the given string equals "false".
func IsFalse(str string) bool {
	return str == "false"
}

// IsValidBool determines whether the given string equals "true" or "false".
func IsValidBool(str string) bool {
	return IsTrue(str) || IsFalse(str)
}
