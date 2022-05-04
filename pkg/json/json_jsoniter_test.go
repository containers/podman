//go:build jsoniter
// +build jsoniter

package json

import (
	"reflect"
	"testing"
)

func TestMarshal(t *testing.T) {
	buffer, err := Marshal(&struct {
		FieldA string
	}{
		"UnitTest A",
	})
	if err != nil {
		t.Errorf("jsoniter Marshal failed: %v", err)
	}
	if !reflect.DeepEqual(buffer, []byte(`{"FieldA":"UnitTest A"}`)) {
		t.Errorf(`Expected "{"FieldA":"UnitTest A"}", got %q`, string(buffer))
	}
}
