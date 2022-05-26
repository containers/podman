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

package release

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"sigs.k8s.io/release-sdk/regex"
	"sigs.k8s.io/release-utils/util"
)

const (
	ReleaseTypeOfficial string = "official"
	ReleaseTypeRC       string = "rc"
	ReleaseTypeBeta     string = "beta"
	ReleaseTypeAlpha    string = "alpha"
)

// Versions specifies the collection of found release versions
type Versions struct {
	prime    string
	official string
	rc       string
	beta     string
	alpha    string
}

// NewReleaseVersions can be used to create a new `*Versions` instance
func NewReleaseVersions(prime, official, rc, beta, alpha string) *Versions {
	return &Versions{
		prime,
		official,
		rc,
		beta,
		alpha,
	}
}

// Prime can be used to get the most prominent release version
func (r *Versions) Prime() string {
	return r.prime
}

// Official can be used to get the ReleaseTypeOfficial from the versions
func (r *Versions) Official() string {
	return r.official
}

// Rc can be used to get the ReleaseTypeRC from the versions
func (r *Versions) RC() string {
	return r.rc
}

// Beta can be used to get the ReleaseTypeBeta from the versions
func (r *Versions) Beta() string {
	return r.beta
}

// Alpha can be used to get the ReleaseTypeAlpha from the versions
func (r *Versions) Alpha() string {
	return r.alpha
}

// String returns a string representation for the release versions
func (r *Versions) String() string {
	sb := &strings.Builder{}
	fmt.Fprintf(sb, "prime: %s", r.prime)
	if r.official != "" {
		fmt.Fprintf(sb, ", %s: %s", ReleaseTypeOfficial, r.official)
	}
	if r.rc != "" {
		fmt.Fprintf(sb, ", %s: %s", ReleaseTypeRC, r.rc)
	}
	if r.beta != "" {
		fmt.Fprintf(sb, ", %s: %s", ReleaseTypeBeta, r.beta)
	}
	if r.alpha != "" {
		fmt.Fprintf(sb, ", %s: %s", ReleaseTypeAlpha, r.alpha)
	}
	return sb.String()
}

// Ordered returns a list of ordered release versions.
func (r *Versions) Ordered() (versions []string) {
	if r.Official() != "" {
		versions = append(versions, r.Official())
	}
	if r.RC() != "" {
		versions = append(versions, r.RC())
	}
	if r.Beta() != "" {
		versions = append(versions, r.Beta())
	}
	if r.Alpha() != "" {
		versions = append(versions, r.Alpha())
	}
	return versions
}

// GenerateReleaseVersion returns the next build versions for the provided parameters
func GenerateReleaseVersion(
	releaseType, version, branch string, branchFromMaster bool,
) (*Versions, error) {
	logrus.Infof(
		"Setting release version for %s (branch: %q, branch from master: %v)",
		version, branch, branchFromMaster,
	)

	// if branch == git.DefaultBranch, version is an alpha or beta
	// if branch == release, version is a rc
	// if branch == release+1, version is an alpha
	v, err := util.TagStringToSemver(version)
	if err != nil {
		return nil, errors.Errorf("invalid formatted version %s", version)
	}

	var label string
	if len(v.Pre) > 0 {
		// alpha, beta, rc
		label = v.Pre[0].String()
	}

	var labelID uint64 = 1
	labelIDAvailable := false
	if len(v.Pre) > 1 && v.Pre[1].IsNum {
		labelIDAvailable = true
		labelID = v.Pre[1].VersionNum + 1
	}

	// releaseVersions.prime is the default release version for this
	// session/type Other labels such as alpha, beta, and rc are set as needed
	// Index ordering is important here as it's how they are processed
	releaseVersions := &Versions{}
	if branchFromMaster {
		branchMatch := regex.BranchRegex.FindStringSubmatch(branch)
		if len(branchMatch) < 3 {
			return nil, errors.Errorf("invalid formatted branch %s", branch)
		}
		branchMajor, err := strconv.Atoi(branchMatch[1])
		if err != nil {
			return nil, errors.Wrapf(
				err, "parsing branch major version %q to int", branchMatch[1],
			)
		}
		branchMinor, err := strconv.Atoi(branchMatch[2])
		if err != nil {
			return nil, errors.Wrapf(
				err, "parsing branch minor version %q to int", branchMatch[2],
			)
		}
		releaseBranch := struct{ major, minor int }{
			major: branchMajor, minor: branchMinor,
		}

		// This is a new branch, set new alpha and RC versions
		releaseVersions.alpha = fmt.Sprintf(
			"v%d.%d.0-alpha.0", releaseBranch.major, releaseBranch.minor+1,
		)

		releaseVersions.rc = fmt.Sprintf(
			"v%d.%d.0-rc.0", releaseBranch.major, releaseBranch.minor,
		)

		releaseVersions.prime = releaseVersions.rc
	} else if strings.HasPrefix(branch, "release-") {
		// Build directly from releaseVersions
		releaseVersions.prime = fmt.Sprintf("v%d.%d", v.Major, v.Minor)

		// If the incoming version is anything bigger than vX.Y.Z, then it's a
		// Jenkin's build version and it stands as is, otherwise increment the
		// patch
		patch := v.Patch
		if !labelIDAvailable {
			patch++
		}
		releaseVersions.prime += fmt.Sprintf(".%d", patch)

		if releaseType == ReleaseTypeOfficial {
			releaseVersions.official = releaseVersions.prime
			// Only primary branches get rc releases
			if regexp.MustCompile(`^release-([0-9]{1,})\.([0-9]{1,})$`).MatchString(branch) {
				releaseVersions.rc = fmt.Sprintf(
					"v%d.%d.%d-rc.0", v.Major, v.Minor, v.Patch+1,
				)
			}
		} else if releaseType == ReleaseTypeRC {
			releaseVersions.rc = fmt.Sprintf(
				"%s-rc.%d", releaseVersions.prime, labelID,
			)
			releaseVersions.prime = releaseVersions.rc
		}
	} else if releaseType == ReleaseTypeBeta {
		releaseVersions.beta = fmt.Sprintf(
			"v%d.%d.%d", v.Major, v.Minor, v.Patch,
		)

		// Enable building beta releases on the main branch.
		// If the last build version was an alpha (x.y.z-alpha.N), set the
		// build
		// label to 'beta' and release version to x.y.z-beta.0.
		//
		// Otherwise, we'll assume this is the next x.y beta, so just
		// increment the
		// beta version e.g., x.y.z-beta.1 --> x.y.z-beta.2
		if label == ReleaseTypeAlpha {
			releaseVersions.beta += fmt.Sprintf("-%s.0", ReleaseTypeBeta)
		} else {
			releaseVersions.beta += fmt.Sprintf("-%s.%d", label, labelID)
		}

		releaseVersions.prime = releaseVersions.beta
	} else {
		// In this code branch, we're implicitly supposed to be at an alpha
		// release. Here, we verify that we're not attempting to cut an
		// alpha release after a beta in the x.y release series.
		//
		// Concretely:
		// We should not be able to cut x.y.z-alpha.N after x.y.z-beta.M
		if label != ReleaseTypeAlpha {
			return nil, errors.Errorf(
				"cannot cut an alpha tag after a non-alpha release %s. %s",
				version,
				"please specify an allowed release type ('beta')",
			)
		}

		releaseVersions.alpha = fmt.Sprintf(
			"v%d.%d.%d-%s.%d", v.Major, v.Minor, v.Patch, label, labelID,
		)
		releaseVersions.prime = releaseVersions.alpha
	}

	logrus.Infof("Found release versions: %+v", releaseVersions.String())
	return releaseVersions, nil
}
