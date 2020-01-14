package hook

import (
	"fmt"
	"testing"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
)

func TestNoMatch(t *testing.T) {
	config := &rspec.Spec{}
	for _, o := range []bool{true, false} {
		or := o
		t.Run(fmt.Sprintf("or %t", or), func(t *testing.T) {
			when := When{Or: or}
			match, err := when.Match(config, map[string]string{}, false)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, false, match)
		})
	}
}

func TestAlways(t *testing.T) {
	config := &rspec.Spec{}
	processStruct := &rspec.Process{
		Args: []string{"/bin/sh", "a", "b"},
	}
	for _, a := range []bool{true, false} {
		always := a
		for _, o := range []bool{true, false} {
			or := o
			for _, p := range []*rspec.Process{processStruct, nil} {
				process := p
				t.Run(fmt.Sprintf("always %t, or %t, has process %t", always, or, process != nil), func(t *testing.T) {
					config.Process = process
					when := When{Always: &always, Or: or}
					match, err := when.Match(config, map[string]string{}, false)
					if err != nil {
						t.Fatal(err)
					}
					assert.Equal(t, always, match)
				})
			}
		}
	}
}

func TestHasBindMountsAnd(t *testing.T) {
	hasBindMounts := true
	when := When{HasBindMounts: &hasBindMounts}
	config := &rspec.Spec{}
	for _, b := range []bool{false, true} {
		containerHasBindMounts := b
		t.Run(fmt.Sprintf("%t", containerHasBindMounts), func(t *testing.T) {
			match, err := when.Match(config, map[string]string{}, containerHasBindMounts)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, containerHasBindMounts, match)
		})
	}
}

func TestHasBindMountsOr(t *testing.T) {
	hasBindMounts := true
	when := When{HasBindMounts: &hasBindMounts, Or: true}
	config := &rspec.Spec{}
	for _, b := range []bool{false, true} {
		containerHasBindMounts := b
		t.Run(fmt.Sprintf("%t", containerHasBindMounts), func(t *testing.T) {
			match, err := when.Match(config, map[string]string{}, containerHasBindMounts)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, containerHasBindMounts, match)
		})
	}
}

func TestAnnotations(t *testing.T) {
	when := When{
		Annotations: map[string]string{
			"^a$": "^b$",
			"^c$": "^d$",
		},
	}
	config := &rspec.Spec{}
	for _, tt := range []struct {
		name        string
		annotations map[string]string
		or          bool
		match       bool
	}{
		{
			name: "matching both, and",
			annotations: map[string]string{
				"a": "b",
				"c": "d",
				"e": "f",
			},
			or:    false,
			match: true,
		},
		{
			name: "matching one, and",
			annotations: map[string]string{
				"a": "b",
			},
			or:    false,
			match: false,
		},
		{
			name: "matching one, or",
			annotations: map[string]string{
				"a": "b",
			},
			or:    true,
			match: true,
		},
		{
			name: "key-only, or",
			annotations: map[string]string{
				"a": "bc",
			},
			or:    true,
			match: false,
		},
		{
			name: "value-only, or",
			annotations: map[string]string{
				"ac": "b",
			},
			or:    true,
			match: false,
		},
	} {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			when.Or = test.or
			match, err := when.Match(config, test.annotations, false)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, test.match, match)
		})
	}
}

func TestCommands(t *testing.T) {
	when := When{
		Commands: []string{
			"^/bin/sh$",
		},
	}
	config := &rspec.Spec{}
	for _, tt := range []struct {
		name    string
		process *rspec.Process
		match   bool
	}{
		{
			name: "good",
			process: &rspec.Process{
				Args: []string{"/bin/sh", "a", "b"},
			},
			match: true,
		},
		{
			name: "extra characters",
			process: &rspec.Process{
				Args: []string{"/bin/shell", "a", "b"},
			},
			match: false,
		},
		{
			name:  "process unset",
			match: false,
		},
	} {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			config.Process = test.process
			match, err := when.Match(config, map[string]string{}, false)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, test.match, match)
		})
	}
}

func TestCommandsEmptyProcessArgs(t *testing.T) {
	when := When{
		Commands: []string{
			"^/bin/sh$",
		},
	}
	config := &rspec.Spec{
		Process: &rspec.Process{},
	}
	_, err := when.Match(config, map[string]string{}, false)
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^process\\.args must have at least one entry$", err.Error())
}

func TestHasBindMountsAndCommands(t *testing.T) {
	hasBindMounts := true
	when := When{
		HasBindMounts: &hasBindMounts,
		Commands: []string{
			"^/bin/sh$",
		},
	}
	config := &rspec.Spec{Process: &rspec.Process{}}
	for _, tt := range []struct {
		name          string
		command       string
		hasBindMounts bool
		or            bool
		match         bool
	}{
		{
			name:          "both, and",
			command:       "/bin/sh",
			hasBindMounts: true,
			or:            false,
			match:         true,
		},
		{
			name:          "both, or",
			command:       "/bin/sh",
			hasBindMounts: true,
			or:            true,
			match:         true,
		},
		{
			name:          "bind, and",
			command:       "/bin/shell",
			hasBindMounts: true,
			or:            false,
			match:         false,
		},
		{
			name:          "bind, or",
			command:       "/bin/shell",
			hasBindMounts: true,
			or:            true,
			match:         true,
		},
		{
			name:          "command, and",
			command:       "/bin/sh",
			hasBindMounts: false,
			or:            false,
			match:         false,
		},
		{
			name:          "command, or",
			command:       "/bin/sh",
			hasBindMounts: false,
			or:            true,
			match:         true,
		},
		{
			name:          "neither, and",
			command:       "/bin/shell",
			hasBindMounts: false,
			or:            false,
			match:         false,
		},
		{
			name:          "neither, or",
			command:       "/bin/shell",
			hasBindMounts: false,
			or:            true,
			match:         false,
		},
	} {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			config.Process.Args = []string{test.command}
			when.Or = test.or
			match, err := when.Match(config, map[string]string{}, test.hasBindMounts)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, test.match, match)
		})
	}
}

func TestInvalidRegexp(t *testing.T) {
	config := &rspec.Spec{Process: &rspec.Process{Args: []string{"/bin/sh"}}}
	for _, tt := range []struct {
		name     string
		when     When
		expected string
	}{
		{
			name:     "invalid-annotation-key",
			when:     When{Annotations: map[string]string{"[": "a"}},
			expected: "^annotation key: error parsing regexp: .*",
		},
		{
			name:     "invalid-annotation-value",
			when:     When{Annotations: map[string]string{"a": "["}},
			expected: "^annotation value: error parsing regexp: .*",
		},
		{
			name:     "invalid-command",
			when:     When{Commands: []string{"["}},
			expected: "^command: error parsing regexp: .*",
		},
	} {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			_, err := test.when.Match(config, map[string]string{"a": "b"}, false)
			if err == nil {
				t.Fatal("unexpected success")
			}
			assert.Regexp(t, test.expected, err.Error())
		})
	}
}
