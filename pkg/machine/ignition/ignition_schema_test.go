//go:build amd64 || arm64

package ignition

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestIgnitionConfigReplaceOmitEmpty verifies that when IgnitionConfig.Replace
// is nil, it is omitted from the JSON output. This is important because an
// empty "replace" object can cause ignition to behave unexpectedly.
func TestIgnitionConfigReplaceOmitEmpty(t *testing.T) {
	// Create an IgnitionConfig with nil Replace (the common case)
	config := IgnitionConfig{
		Merge:   nil,
		Replace: nil,
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("failed to marshal IgnitionConfig: %v", err)
	}

	jsonStr := string(data)

	// The "replace" field should not appear in the JSON output
	if strings.Contains(jsonStr, "replace") {
		t.Errorf("expected 'replace' to be omitted from JSON when nil, got: %s", jsonStr)
	}
}

// TestResourceVerificationOmitEmpty verifies that when Resource.Verification
// is nil, it is omitted from the JSON output.
func TestResourceVerificationOmitEmpty(t *testing.T) {
	// Create a Resource with nil Verification
	resource := Resource{
		Source:       nil,
		Verification: nil,
	}

	data, err := json.Marshal(resource)
	if err != nil {
		t.Fatalf("failed to marshal Resource: %v", err)
	}

	jsonStr := string(data)

	// The "verification" field should not appear in the JSON output
	if strings.Contains(jsonStr, "verification") {
		t.Errorf("expected 'verification' to be omitted from JSON when nil, got: %s", jsonStr)
	}
}

// TestIgnitionConfigReplaceWithValue verifies that when IgnitionConfig.Replace
// has a value, it is included in the JSON output.
func TestIgnitionConfigReplaceWithValue(t *testing.T) {
	source := "https://example.com/config.ign"
	config := IgnitionConfig{
		Replace: &Resource{
			Source: &source,
		},
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("failed to marshal IgnitionConfig: %v", err)
	}

	jsonStr := string(data)

	// The "replace" field should appear in the JSON output
	if !strings.Contains(jsonStr, "replace") {
		t.Errorf("expected 'replace' to be present in JSON when set, got: %s", jsonStr)
	}

	// The source should be in the output
	if !strings.Contains(jsonStr, source) {
		t.Errorf("expected source URL to be present in JSON, got: %s", jsonStr)
	}
}

// TestResourceVerificationWithValue verifies that when Resource.Verification
// has a value, it is included in the JSON output.
func TestResourceVerificationWithValue(t *testing.T) {
	hash := "sha512-abc123"
	resource := Resource{
		Verification: &Verification{
			Hash: &hash,
		},
	}

	data, err := json.Marshal(resource)
	if err != nil {
		t.Fatalf("failed to marshal Resource: %v", err)
	}

	jsonStr := string(data)

	// The "verification" field should appear in the JSON output
	if !strings.Contains(jsonStr, "verification") {
		t.Errorf("expected 'verification' to be present in JSON when set, got: %s", jsonStr)
	}

	// The hash should be in the output
	if !strings.Contains(jsonStr, hash) {
		t.Errorf("expected hash to be present in JSON, got: %s", jsonStr)
	}
}

// TestFullConfigOmitsEmptyOptionalFields verifies that a full Config struct
// with empty optional fields produces clean JSON without empty objects.
func TestFullConfigOmitsEmptyOptionalFields(t *testing.T) {
	config := Config{
		Ignition: Ignition{
			Version: "3.2.0",
			Config:  IgnitionConfig{
				// Both Merge and Replace are nil/empty
			},
		},
		Passwd:  Passwd{},
		Storage: Storage{},
		Systemd: Systemd{},
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("failed to marshal Config: %v", err)
	}

	jsonStr := string(data)

	// Should not contain "replace" when it's nil
	if strings.Contains(jsonStr, "\"replace\":{}") {
		t.Errorf("expected empty 'replace' object to be omitted, got: %s", jsonStr)
	}

	// Should contain the version
	if !strings.Contains(jsonStr, "3.2.0") {
		t.Errorf("expected version to be present in JSON, got: %s", jsonStr)
	}
}
