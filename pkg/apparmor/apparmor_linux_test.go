// +build linux,apparmor

package apparmor

import (
	"os"
	"testing"
)

type versionExpected struct {
	output  string
	version int
}

func TestParseAAParserVersion(t *testing.T) {
	if !IsEnabled() {
		t.Skip("AppArmor disabled: skipping tests")
	}
	versions := []versionExpected{
		{
			output: `AppArmor parser version 2.10
Copyright (C) 1999-2008 Novell Inc.
Copyright 2009-2012 Canonical Ltd.

`,
			version: 210000,
		},
		{
			output: `AppArmor parser version 2.8
Copyright (C) 1999-2008 Novell Inc.
Copyright 2009-2012 Canonical Ltd.

`,
			version: 208000,
		},
		{
			output: `AppArmor parser version 2.20
Copyright (C) 1999-2008 Novell Inc.
Copyright 2009-2012 Canonical Ltd.

`,
			version: 220000,
		},
		{
			output: `AppArmor parser version 2.05
Copyright (C) 1999-2008 Novell Inc.
Copyright 2009-2012 Canonical Ltd.

`,
			version: 205000,
		},
		{
			output: `AppArmor parser version 2.9.95
Copyright (C) 1999-2008 Novell Inc.
Copyright 2009-2012 Canonical Ltd.

`,
			version: 209095,
		},
		{
			output: `AppArmor parser version 3.14.159
Copyright (C) 1999-2008 Novell Inc.
Copyright 2009-2012 Canonical Ltd.

`,
			version: 314159,
		},
	}

	for _, v := range versions {
		version, err := parseAAParserVersion(v.output)
		if err != nil {
			t.Fatalf("expected error to be nil for %#v, got: %v", v, err)
		}
		if version != v.version {
			t.Fatalf("expected version to be %d, was %d, for: %#v\n", v.version, version, v)
		}
	}
}

func TestInstallDefault(t *testing.T) {
	profile := "libpod-default-testing"
	aapath := "/sys/kernel/security/apparmor/"

	if _, err := os.Stat(aapath); err != nil {
		t.Skip("AppArmor isn't available in this environment")
	}

	// removes `profile`
	removeProfile := func() error {
		path := aapath + ".remove"

		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = f.WriteString(profile)
		return err
	}

	// makes sure `profile` is loaded according to `state`
	checkLoaded := func(state bool) {
		loaded, err := IsLoaded(profile)
		if err != nil {
			t.Fatalf("Error searching AppArmor profile '%s': %v", profile, err)
		}
		if state != loaded {
			if state {
				t.Fatalf("AppArmor profile '%s' isn't loaded but should", profile)
			} else {
				t.Fatalf("AppArmor profile '%s' is loaded but shouldn't", profile)
			}
		}
	}

	// test installing the profile
	if err := InstallDefault(profile); err != nil {
		t.Fatalf("Couldn't install AppArmor profile '%s': %v", profile, err)
	}
	checkLoaded(true)

	// remove the profile and check again
	if err := removeProfile(); err != nil {
		t.Fatalf("Couldn't remove AppArmor profile '%s': %v", profile, err)
	}
	checkLoaded(false)
}
