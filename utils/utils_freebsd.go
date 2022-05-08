//go:build freebsd
// +build freebsd

package utils

import "errors"

func RunUnderSystemdScope(pid int, slice string, unitName string) error {
	return errors.New("not implemented for freebsd")
}

func MoveUnderCgroupSubtree(subtree string) error {
	return errors.New("not implemented for freebsd")
}

func GetOwnCgroup() (string, error) {
	return "", errors.New("not implemented for freebsd")
}

func GetCgroupProcess(pid int) (string, error) {
	return "", errors.New("not implemented for freebsd")
}
