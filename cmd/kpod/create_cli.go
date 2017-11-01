package main

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

func getAllLabels(cli *cli.Context) (map[string]string, error) {
	var labelValues []string
	labels := make(map[string]string)
	labelValues, labelErr := readKVStrings(cli.StringSlice("label-file"), cli.StringSlice("label"))
	if labelErr != nil {
		return labels, errors.Wrapf(labelErr, "unable to process labels from --label and label-file")
	}
	// Process KEY=VALUE stringslice in string map for WithLabels func
	if len(labelValues) > 0 {
		for _, i := range labelValues {
			spliti := strings.Split(i, "=")
			if len(spliti) > 1 {
				return labels, errors.Errorf("labels must be in KEY=VALUE format: %s is invalid", i)
			}
			labels[spliti[0]] = spliti[1]
		}
	}
	return labels, nil
}

func getAllEnvironmentVariables(cli *cli.Context) ([]string, error) {
	env, err := readKVStrings(cli.StringSlice("env-file"), cli.StringSlice("env"))
	if err != nil {
		return []string{}, errors.Wrapf(err, "unable to process variables from --env and --env-file")
	}
	// Add default environment variables if nothing defined
	if len(env) == 0 {
		env = append(env, defaultEnvVariables...)
	}
	return env, nil
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
