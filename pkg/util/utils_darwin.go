//+build darwin

package util

import (
	"github.com/pkg/errors"
)

func GetContainerPidInformationDescriptors() ([]string, error) {
	return []string{}, errors.New("this function is not supported on darwin")
}
