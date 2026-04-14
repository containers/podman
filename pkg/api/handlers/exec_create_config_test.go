package handlers_test

import (
	"encoding/json"
	"testing"

	"github.com/containers/podman/v6/pkg/api/handlers"
)

// TestExecCreateConfigDetachKeys verifies that DetachKeys *string correctly
// distinguishes "field absent" (nil) from "explicitly set to empty string" (*"").
// This is the regression test for the bug where DetachKeys:"" was silently
// ignored because the zero value of string is indistinguishable from absent.
func TestExecCreateConfigDetachKeys(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantNil bool
		wantVal string
	}{
		{
			name:    "empty body - DetachKeys should be nil",
			body:    `{}`,
			wantNil: true,
		},
		{
			name:    "field absent - DetachKeys should be nil",
			body:    `{"AttachStdout":true}`,
			wantNil: true,
		},
		{
			name:    "field set to empty string - DetachKeys should be non-nil empty",
			body:    `{"DetachKeys":""}`,
			wantNil: false,
			wantVal: "",
		},
		{
			name:    "field set to value - DetachKeys should be non-nil with value",
			body:    `{"DetachKeys":"ctrl-p,ctrl-q"}`,
			wantNil: false,
			wantVal: "ctrl-p,ctrl-q",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg handlers.ExecCreateConfig
			if err := json.Unmarshal([]byte(tt.body), &cfg); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if tt.wantNil {
				if cfg.DetachKeys != nil {
					t.Errorf("expected DetachKeys to be nil, got %q", *cfg.DetachKeys)
				}
			} else {
				if cfg.DetachKeys == nil {
					t.Fatal("expected DetachKeys to be non-nil")
				}
				if *cfg.DetachKeys != tt.wantVal {
					t.Errorf("DetachKeys = %q, want %q", *cfg.DetachKeys, tt.wantVal)
				}
			}
		})
	}
}
