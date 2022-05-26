/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gcli

import (
	"github.com/pkg/errors"

	"sigs.k8s.io/release-utils/command"
)

const (
	// GCloudExecutable is the name of the Google Cloud SDK executable
	GCloudExecutable = "gcloud"
	gsutilExecutable = "gsutil"
)

// PreCheck checks if all requirements are fulfilled to run this package and
// all sub-packages
func PreCheck() error {
	for _, e := range []string{
		GCloudExecutable,
		gsutilExecutable,
	} {
		if !command.Available(e) {
			return errors.Errorf(
				"%s executable is not available in $PATH", e,
			)
		}
	}

	return nil
}

// GCloud can be used to run a 'gcloud' command
func GCloud(args ...string) error {
	return command.New(GCloudExecutable, args...).RunSilentSuccess()
}

// GCloudOutput can be used to run a 'gcloud' command while capturing its output
func GCloudOutput(args ...string) (string, error) {
	stream, err := command.New(GCloudExecutable, args...).RunSilentSuccessOutput()
	if err != nil {
		return "", errors.Wrapf(err, "executing %s", GCloudExecutable)
	}
	return stream.OutputTrimNL(), nil
}

// GSUtil can be used to run a 'gsutil' command
func GSUtil(args ...string) error {
	return command.New(gsutilExecutable, args...).RunSilentSuccess()
}

// GSUtilOutput can be used to run a 'gsutil' command while capturing its output
func GSUtilOutput(args ...string) (string, error) {
	stream, err := command.New(gsutilExecutable, args...).RunSilentSuccessOutput()
	if err != nil {
		return "", errors.Wrapf(err, "executing %s", gsutilExecutable)
	}
	return stream.OutputTrimNL(), nil
}

// GSUtilStatus can be used to run a 'gsutil' command while capturing its status
func GSUtilStatus(args ...string) (*command.Status, error) {
	return command.New(gsutilExecutable, args...).Run()
}
