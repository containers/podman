package utils

import (
	"net"
	"strconv"

	"github.com/pkg/errors"
)

// Find a random, open port on the host
func GetRandomPort() (int, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, errors.Wrapf(err, "unable to get free TCP port")
	}
	defer l.Close()
	_, randomPort, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return 0, errors.Wrapf(err, "unable to determine free port")
	}
	rp, err := strconv.Atoi(randomPort)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to convert random port to int")
	}
	return rp, nil
}
