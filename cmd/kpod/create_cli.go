package main

import (
	"strings"

	"github.com/pkg/errors"
)

func getAllLabels(labelFile, inputLabels []string) (map[string]string, error) {
	labels := make(map[string]string)
	labelErr := readKVStrings(labels, labelFile, inputLabels)
	if labelErr != nil {
		return labels, errors.Wrapf(labelErr, "unable to process labels from --label and label-file")
	}
	return labels, nil
}

func convertStringSliceToMap(strSlice []string, delimiter string) (map[string]string, error) {
	sysctl := make(map[string]string)
	for _, inputSysctl := range strSlice {
		values := strings.Split(inputSysctl, delimiter)
		if len(values) < 2 {
			return sysctl, errors.Errorf("%s in an invalid sysctl value", inputSysctl)
		}
		sysctl[values[0]] = values[1]
	}
	return sysctl, nil
}
