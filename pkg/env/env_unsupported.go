// +build !linux,!darwin

package env

func ParseSlice(s []string) (map[string]string, error) {
	m := make(map[string]string)
	return m, nil
}
