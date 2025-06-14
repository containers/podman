//go:build !remote

package utils

import (
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestErrorEncoderFuncOmit(t *testing.T) {
	data, err := json.Marshal(struct {
		Err  error   `json:"err,omitempty"`
		Errs []error `json:"errs,omitempty"`
	}{})
	if err != nil {
		t.Fatal(err)
	}

	dataAsMap := make(map[string]interface{})
	err = json.Unmarshal(data, &dataAsMap)
	if err != nil {
		t.Fatal(err)
	}

	_, ok := dataAsMap["err"]
	if ok {
		t.Errorf("the `err` field should have been omitted")
	}
	_, ok = dataAsMap["errs"]
	if ok {
		t.Errorf("the `errs` field should have been omitted")
	}

	dataAsMap = make(map[string]interface{})
	data, err = json.Marshal(struct {
		Err  error   `json:"err"`
		Errs []error `json:"errs"`
	}{})
	if err != nil {
		t.Fatal(err)
	}

	err = json.Unmarshal(data, &dataAsMap)
	if err != nil {
		t.Fatal(err)
	}

	_, ok = dataAsMap["err"]
	if !ok {
		t.Errorf("the `err` field shouldn't have been omitted")
	}
	_, ok = dataAsMap["errs"]
	if !ok {
		t.Errorf("the `errs` field shouldn't have been omitted")
	}
}

func TestWriteJSONNoHTMLEscape(t *testing.T) {
	// Test that WriteJSON does not HTML-escape JSON content
	// This test verifies the fix for issue #17769

	recorder := httptest.NewRecorder()

	// Test data with characters that would be HTML-escaped
	testData := map[string]string{
		"message": "Hello <world> & \"friends\"",
		"script":  "<script>alert('test')</script>",
		"url":     "https://example.com/path?param=value&other=<test>",
	}

	WriteJSON(recorder, 200, testData)

	// Check response headers
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}

	// Check that response contains unescaped characters
	body := recorder.Body.String()

	// These characters should NOT be HTML-escaped in JSON responses
	// (but quotes are still properly JSON-escaped)
	expectedUnescaped := []string{
		"<world>",
		"&",
		"\\\"friends\\\"", // JSON-escaped quotes, not HTML-escaped
		"<script>",
		"<test>",
	}

	for _, expected := range expectedUnescaped {
		if !strings.Contains(body, expected) {
			t.Errorf("Expected unescaped string '%s' in response body, got: %s", expected, body)
		}
	}

	// Verify we can parse the JSON back
	var parsed map[string]string
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Errorf("Failed to parse JSON response: %v", err)
	}

	// Verify the data matches what we sent
	if !reflect.DeepEqual(parsed, testData) {
		t.Errorf("Parsed message doesn't match original: got %v, want %v", parsed, testData)
	}
}
