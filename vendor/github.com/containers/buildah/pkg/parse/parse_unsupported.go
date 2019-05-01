// +build !linux,!darwin

package parse

func getDefaultProcessLimits() []string {
	return []string{}
}
