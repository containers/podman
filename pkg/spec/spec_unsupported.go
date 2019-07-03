//+build !linux

package createconfig

func getHostRlimits() ([]systemUlimit, error) {
	return nil, nil
}
