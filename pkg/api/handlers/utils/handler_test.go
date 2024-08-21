//go:build !remote

package utils

import (
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
