package main

import (
	"os/user"
	"testing"
)

func skipTestIfNotRoot(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Skip("Could not determine user.  Running without root may cause tests to fail")
	} else if u.Uid != "0" {
		t.Skip("tests will fail unless run as root")
	}
}
