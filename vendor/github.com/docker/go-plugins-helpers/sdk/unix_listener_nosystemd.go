// +build linux freebsd
// +build nosystemd

package sdk

import "net"

func setupSocketActivation() (net.Listener, error) {
	return nil, nil
}
