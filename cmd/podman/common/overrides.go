package common

import (
	"os"

	"github.com/sirupsen/logrus"
)

// Override path to signature policy if it is not set by shell caller
func OverrideSignaturePolicyIfEmpty(value *string) {
	if *value == "" {
		if envSigPath, ok := os.LookupEnv("CONTAINERS_POLICY_JSON"); ok {
			if _, err := os.Stat(envSigPath); err == nil {
				logrus.Debugf("using signature policy file from CONTAINERS_POLICY_JSON environment variable: %s", envSigPath)
				*value = envSigPath
			}
		}
	}
}
