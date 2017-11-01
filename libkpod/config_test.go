package libkpod

import (
	"io/ioutil"
	"os"
	"testing"
)

// TestConfigToFile ensures Config.ToFile(..) encodes and writes out
// a Config instance toa a file on disk.
func TestConfigToFile(t *testing.T) {
	// Test with a default configuration
	c := DefaultConfig()
	tmpfile, err := ioutil.TempFile("", "config")
	if err != nil {
		t.Fatalf("Unable to create temporary file: %+v", err)
	}
	// Clean up temporary file
	defer os.Remove(tmpfile.Name())

	// Make the ToFile calls
	err = c.ToFile(tmpfile.Name())
	// Make sure no errors occurred while populating the file
	if err != nil {
		t.Fatalf("Unable to write to temporary file: %+v", err)
	}

	// Make sure the file is on disk
	if _, err := os.Stat(tmpfile.Name()); os.IsNotExist(err) {
		t.Fatalf("The config file was not written to disk: %+v", err)
	}
}

// TestConfigUpdateFromFile ensures Config.UpdateFromFile(..) properly
// updates an already create Config instancec with new data.
func TestConfigUpdateFromFile(t *testing.T) {
	// Test with a default configuration
	c := DefaultConfig()
	// Make the ToFile calls
	err := c.UpdateFromFile("testdata/config.toml")
	// Make sure no errors occurred while populating from the file
	if err != nil {
		t.Fatalf("Unable update config from file: %+v", err)
	}

	// Check fields that should have changed after UpdateFromFile
	if c.Storage != "overlay2" {
		t.Fatalf("Update failed. Storage did not change to overlay2")
	}

	if c.RuntimeConfig.PidsLimit != 2048 {
		t.Fatalf("Update failed. RuntimeConfig.PidsLimit did not change to 2048")
	}
}
