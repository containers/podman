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

package notes

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/saschagrunert/go-modiff/pkg/modiff"

	"sigs.k8s.io/release-sdk/git"
)

type Dependencies struct {
	moDiff MoDiff
}

func NewDependencies() *Dependencies {
	return &Dependencies{&moDiff{}}
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate . MoDiff
type MoDiff interface {
	Run(*modiff.Config) (string, error)
}

type moDiff struct{}

func (m *moDiff) Run(config *modiff.Config) (string, error) {
	return modiff.Run(config)
}

// SetMoDiff can be used to set the internal MoDiff implementation
func (d *Dependencies) SetMoDiff(moDiff MoDiff) {
	d.moDiff = moDiff
}

// Changes collects the dependency change report as markdown between
// both provided revisions. The function errors if anything went wrong.
func (d *Dependencies) Changes(from, to string) (string, error) {
	return d.ChangesForURL(git.GetDefaultKubernetesRepoURL(), from, to)
}

// Changes collects the dependency change report as markdown between
// both provided revisions for the specified repository URL. The function
// errors if anything went wrong.
func (d *Dependencies) ChangesForURL(url, from, to string) (string, error) {
	config := modiff.NewConfig(
		strings.TrimPrefix(url, "https://"), from, to, true, 2,
	)

	res, err := d.moDiff.Run(config)
	if err != nil {
		return "", errors.Wrap(err, "getting dependency changes")
	}

	return res, nil
}
