// +build linux

package chroot

func dedupeStringSlice(slice []string) []string {
	done := make([]string, 0, len(slice))
	m := make(map[string]struct{})
	for _, s := range slice {
		if _, present := m[s]; !present {
			m[s] = struct{}{}
			done = append(done, s)
		}
	}
	return done
}
