package env

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlice(t *testing.T) {
	goodMap := make(map[string]string, 0)
	goodMap["apple"] = "red"
	goodMap["banana"] = "yellow"
	goodMap["pear"] = ""
	goodResult := []string{"apple=red", "banana=yellow", "pear"}
	type args struct {
		m map[string]string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Good",
			args: args{
				m: goodMap,
			},
			want: goodResult,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatchf(t, Slice(tt.args.m), tt.want, "Slice() = %v, want %v", Slice(tt.args.m), tt.want)
		})
	}
}

func TestJoin(t *testing.T) {
	firstMap := make(map[string]string, 0)
	firstMap["apple"] = "red"
	secondMap := make(map[string]string, 0)
	secondMap["banana"] = "yellow"
	goodResult := make(map[string]string, 0)
	goodResult["apple"] = "red"
	goodResult["banana"] = "yellow"
	overrideResult := make(map[string]string, 0)
	overrideResult["apple"] = "green"
	overrideResult["banana"] = "yellow"
	overrideMap := make(map[string]string, 0)
	overrideMap["banana"] = "yellow"
	overrideMap["apple"] = "green"
	type args struct {
		base     map[string]string
		override map[string]string
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "GoodJoin",
			args: args{
				base:     firstMap,
				override: secondMap,
			},
			want: goodResult,
		},
		{
			name: "GoodOverride",
			args: args{
				base:     firstMap,
				override: overrideMap,
			},
			want: overrideResult,
		},
		{
			name: "EmptyOverride",
			args: args{
				base:     firstMap,
				override: nil,
			},
			want: firstMap,
		},
		{
			name: "EmptyBase",
			args: args{
				base:     nil,
				override: firstMap,
			},
			want: firstMap,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Join(tt.args.base, tt.args.override)
			assert.EqualValuesf(t, got, tt.want, "Join() = %v, want %v", got, tt.want)
		})
	}
}

func createTmpFile(content string) (string, error) {
	tmpfile, err := os.CreateTemp(os.TempDir(), "podman-test-parse-env-")
	if err != nil {
		return "", err
	}

	if _, err := tmpfile.WriteString(content); err != nil {
		return "", err
	}
	if err := tmpfile.Close(); err != nil {
		return "", err
	}
	return tmpfile.Name(), nil
}

func Test_ParseFile(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		separator string // = or *
		value     string

		// environment variable
		envKey   string
		envValue string

		expectedKey   string
		expectedValue string
		success       bool
	}{
		{
			name:      "Good",
			key:       "Key",
			separator: "=",
			value:     "Value1",
			success:   true,
		},
		{
			name:      "HasDoubleQuotesWithSingleLine",
			key:       "Key2",
			separator: "=",
			value:     `"Value2"`,
			success:   true,
		},
		{
			name:      "HasSingleQuotesWithSingleLine",
			key:       "Key3",
			separator: "=",
			value:     `'Value3'`,
			success:   true,
		},
		{
			name:      "KeepValueSpace",
			key:       "Key4",
			separator: "=",
			value:     "  Value4  ",
			success:   true,
		},
		{
			name:        "RemoveKeySpace",
			key:         "  Key5  ",
			separator:   "=",
			expectedKey: "Key5",
			value:       "Value5",
			success:     true,
		},
		{
			name:          "NoValue",
			key:           "Key6",
			separator:     "=",
			value:         "",
			envValue:      "Value6",
			expectedValue: "Value6",
			success:       true,
		},
		{
			name:          "FromEnv",
			key:           "Key7",
			separator:     "=",
			value:         "",
			envValue:      "Value7",
			expectedValue: "Value7",
			success:       true,
		},
		{
			name:          "OnlyKey",
			key:           "Key8",
			separator:     "",
			value:         "",
			envValue:      "Value8",
			expectedValue: "Value8",
			success:       true,
		},
		{
			name:          "GlobKey",
			key:           "Key9",
			separator:     "*",
			value:         "",
			envKey:        "Key9999",
			envValue:      "Value9",
			expectedKey:   "Key9999",
			expectedValue: "Value9",
			success:       true,
		},
		{
			name:          "InvalidGlobKey",
			key:           "Key10*",
			separator:     "=",
			value:         "1",
			envKey:        "Key1010",
			envValue:      "Value10",
			expectedKey:   "Key10*",
			expectedValue: "1",
			success:       true,
		},
		{
			name:          "MultilineWithDoubleQuotes",
			key:           "Key11",
			separator:     "=",
			value:         "\"First line1\nlast line1\"",
			expectedValue: "First line1\nlast line1",
			success:       true,
		},
		{
			name:          "MultilineWithSingleQuotes",
			key:           "Key12",
			separator:     "=",
			value:         "'First line2\nlast line2'",
			expectedValue: "First line2\nlast line2",
			success:       true,
		},
		{
			name:      "Has%",
			key:       "BASH_FUNC__fmt_ctx%%",
			separator: "=",
			value:     "() { echo 1; }",
			success:   true,
		},
		{
			name:          "Export syntax",
			key:           "export Key13",
			separator:     "=",
			value:         "Value13",
			expectedKey:   "Key13",
			expectedValue: "Value13",
			success:       true,
		},
		{
			name:      "NoValueAndNoEnv",
			key:       "Key14",
			separator: "=",
			value:     "",
			success:   false,
		},
		{
			name:      "OnlyValue",
			key:       "",
			separator: "=",
			value:     "Value",
			success:   false,
		},
		{
			name:      "OnlyDelim",
			key:       "",
			separator: "=",
			value:     "",
			success:   false,
		},
		{
			name:      "Comment",
			key:       "#aaaa",
			separator: "",
			value:     "",
			success:   false,
		},
	}

	content := ""
	for _, tt := range tests {
		content += fmt.Sprintf("%s%s%s\n", tt.key, tt.separator, tt.value)

		if tt.envValue != "" {
			key := tt.key
			if tt.envKey != "" {
				key = tt.envKey
			}
			t.Setenv(key, tt.envValue)
		}
	}
	tFile, err := createTmpFile(content)
	defer os.Remove(tFile)
	assert.NoError(t, err)

	env, err := ParseFile(tFile)
	assert.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := tt.key
			if tt.expectedKey != "" {
				key = tt.expectedKey
			}
			val, ok := env[key]
			if ok && !tt.success {
				t.Errorf("not should set key:%s ", tt.key)
				return
			} else if !ok && tt.success {
				t.Errorf("should set key:%s ", tt.key)
				return
			}

			if tt.success {
				value := tt.value
				if tt.expectedValue != "" {
					value = tt.expectedValue
				}
				assert.Equal(t, value, val, "value should be equal")
			}
		})
	}
}

func Test_parseEnv(t *testing.T) {
	good := make(map[string]string)

	type args struct {
		env  map[string]string
		line string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Good",
			args: args{
				env:  good,
				line: "apple=red",
			},
			wantErr: false,
		},
		{
			name: "GoodNoValue",
			args: args{
				env:  good,
				line: "apple=",
			},
			wantErr: false,
		},
		{
			name: "GoodNoKeyNoValue",
			args: args{
				env:  good,
				line: "=",
			},
			wantErr: true,
		},
		{
			name: "GoodOnlyKey",
			args: args{
				env:  good,
				line: "apple",
			},
			wantErr: false,
		},
		{
			name: "BadNoKey",
			args: args{
				env:  good,
				line: "=foobar",
			},
			wantErr: true,
		},
		{
			name: "BadOnlyDelim",
			args: args{
				env:  good,
				line: "=",
			},
			wantErr: true,
		},
		{
			name: "MultilineWithBackticksQuotes",
			args: args{
				env:  good,
				line: "apple=`foo\nbar`",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := parseEnv(tt.args.env, tt.args.line, true); (err != nil) != tt.wantErr {
				t.Errorf("parseEnv() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
