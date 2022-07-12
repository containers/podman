//go:build windows
// +build windows

package utils

import "errors"

func RunUnderSystemdScope(pid int, slice string, unitName string) error {
	return errors.New("not implemented for windows")
}

func MoveUnderCgroupSubtree(subtree string) error {
	return errors.New("not implemented for windows")
}

func GetOwnCgroup() (string, error) {
	return "", errors.New("not implemented for windows")
}

func GetOwnCgroupDisallowRoot() (string, error) {
	return "", errors.New("not implemented for windows")
}

func GetCgroupProcess(pid int) (string, error) {
	return "", errors.New("not implemented for windows")
}
