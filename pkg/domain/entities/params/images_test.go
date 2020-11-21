package params

import (
	"encoding/json"
	"strconv"
	"testing"
)

func TestImagesListOptions_ToString(t *testing.T) {
	_true := true
	o := ImagesListOptions{
		All:     &_true,
		Digests: &_true,
		Ids:     []string{"one", "two"},
		Filters: map[string][]string{
			"before": {"today"},
		},
	}

	for _, fname := range []string{"All", "Digests"} {
		v := o.ToString(fname)
		if v != "true" {
			t.Errorf("Failed formatting %s: %q != %q", fname, v, strconv.FormatBool(true))
		}
	}

	v := o.ToString("Ids")
	if v != "one,two" {
		t.Errorf("Failed formatting Ids: %q != %q", v, "one,two")
	}

	v = o.ToString("Filters")
	j, _ := json.Marshal(o.Filters)
	if v != string(j) {
		t.Errorf("Failed formatting Filters: %q != %q", v, string(j))
	}
}

func TestImagesListOptions_Changed(t *testing.T) {
	_true := true
	_false := false
	o := ImagesListOptions{
		All:     &_true,
		Digests: &_false,
	}

	if !o.Changed("All") {
		t.Errorf("Failed to see state change for All")
	}
	if !o.Changed("Digests") {
		t.Errorf("Failed to see state change Digests")
	}
	if o.Changed("Filters") {
		t.Errorf("Reported state change for Filters")
	}

	o1 := ImagesListOptions{
		Filters: map[string][]string{},
	}
	if !o1.Changed("Filters") {
		t.Errorf("Failed to see state change Filters")
	}
}
