package exec

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestRuntimeConfigFilter(t *testing.T) {
	unexpectedEndOfJSONInput := json.Unmarshal([]byte("{\n"), nil) //nolint
	fileMode := os.FileMode(0600)
	rootUint32 := uint32(0)
	binUser := int(1)
	for _, tt := range []struct {
		name              string
		contextTimeout    time.Duration
		hooks             []spec.Hook
		input             *spec.Spec
		expected          *spec.Spec
		expectedHookError string
		expectedRunError  error
	}{
		{
			name: "no-op",
			hooks: []spec.Hook{
				{
					Path: path,
					Args: []string{"sh", "-c", "cat"},
				},
			},
			input: &spec.Spec{
				Version: "1.0.0",
				Root: &spec.Root{
					Path: "rootfs",
				},
			},
			expected: &spec.Spec{
				Version: "1.0.0",
				Root: &spec.Root{
					Path: "rootfs",
				},
			},
		},
		{
			name: "device injection",
			hooks: []spec.Hook{
				{
					Path: path,
					Args: []string{"sh", "-c", `sed 's|\("gid":0}\)|\1,{"path": "/dev/sda","type":"b","major":8,"minor":0,"fileMode":384,"uid":0,"gid":0}|'`},
				},
			},
			input: &spec.Spec{
				Version: "1.0.0",
				Root: &spec.Root{
					Path: "rootfs",
				},
				Linux: &spec.Linux{
					Devices: []spec.LinuxDevice{
						{
							Path:     "/dev/fuse",
							Type:     "c",
							Major:    10,
							Minor:    229,
							FileMode: &fileMode,
							UID:      &rootUint32,
							GID:      &rootUint32,
						},
					},
				},
			},
			expected: &spec.Spec{
				Version: "1.0.0",
				Root: &spec.Root{
					Path: "rootfs",
				},
				Linux: &spec.Linux{
					Devices: []spec.LinuxDevice{
						{
							Path:     "/dev/fuse",
							Type:     "c",
							Major:    10,
							Minor:    229,
							FileMode: &fileMode,
							UID:      &rootUint32,
							GID:      &rootUint32,
						},
						{
							Path:     "/dev/sda",
							Type:     "b",
							Major:    8,
							Minor:    0,
							FileMode: &fileMode,
							UID:      &rootUint32,
							GID:      &rootUint32,
						},
					},
				},
			},
		},
		{
			name: "chaining",
			hooks: []spec.Hook{
				{
					Path: path,
					Args: []string{"sh", "-c", `sed 's|\("gid":0}\)|\1,{"path": "/dev/sda","type":"b","major":8,"minor":0,"fileMode":384,"uid":0,"gid":0}|'`},
				},
				{
					Path: path,
					Args: []string{"sh", "-c", `sed 's|/dev/sda|/dev/sdb|'`},
				},
			},
			input: &spec.Spec{
				Version: "1.0.0",
				Root: &spec.Root{
					Path: "rootfs",
				},
				Linux: &spec.Linux{
					Devices: []spec.LinuxDevice{
						{
							Path:     "/dev/fuse",
							Type:     "c",
							Major:    10,
							Minor:    229,
							FileMode: &fileMode,
							UID:      &rootUint32,
							GID:      &rootUint32,
						},
					},
				},
			},
			expected: &spec.Spec{
				Version: "1.0.0",
				Root: &spec.Root{
					Path: "rootfs",
				},
				Linux: &spec.Linux{
					Devices: []spec.LinuxDevice{
						{
							Path:     "/dev/fuse",
							Type:     "c",
							Major:    10,
							Minor:    229,
							FileMode: &fileMode,
							UID:      &rootUint32,
							GID:      &rootUint32,
						},
						{
							Path:     "/dev/sdb",
							Type:     "b",
							Major:    8,
							Minor:    0,
							FileMode: &fileMode,
							UID:      &rootUint32,
							GID:      &rootUint32,
						},
					},
				},
			},
		},
		{
			name:           "context timeout",
			contextTimeout: time.Duration(1) * time.Second,
			hooks: []spec.Hook{
				{
					Path: path,
					Args: []string{"sh", "-c", "sleep 2"},
				},
			},
			input: &spec.Spec{
				Version: "1.0.0",
				Root: &spec.Root{
					Path: "rootfs",
				},
			},
			expected: &spec.Spec{
				Version: "1.0.0",
				Root: &spec.Root{
					Path: "rootfs",
				},
			},
			expectedHookError: "^executing \\[sh -c sleep 2]: signal: killed$",
			expectedRunError:  context.DeadlineExceeded,
		},
		{
			name: "hook timeout",
			hooks: []spec.Hook{
				{
					Path:    path,
					Args:    []string{"sh", "-c", "sleep 2"},
					Timeout: &binUser,
				},
			},
			input: &spec.Spec{
				Version: "1.0.0",
				Root: &spec.Root{
					Path: "rootfs",
				},
			},
			expected: &spec.Spec{
				Version: "1.0.0",
				Root: &spec.Root{
					Path: "rootfs",
				},
			},
			expectedHookError: "^executing \\[sh -c sleep 2]: signal: killed$",
			expectedRunError:  context.DeadlineExceeded,
		},
		{
			name: "invalid JSON",
			hooks: []spec.Hook{
				{
					Path: path,
					Args: []string{"sh", "-c", "echo '{'"},
				},
			},
			input: &spec.Spec{
				Version: "1.0.0",
				Root: &spec.Root{
					Path: "rootfs",
				},
			},
			expected: &spec.Spec{
				Version: "1.0.0",
				Root: &spec.Root{
					Path: "rootfs",
				},
			},
			expectedRunError: unexpectedEndOfJSONInput,
		},
	} {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			if test.contextTimeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, test.contextTimeout)
				defer cancel()
			}
			hookErr, err := RuntimeConfigFilter(ctx, test.hooks, test.input, DefaultPostKillTimeout)
			assert.Equal(t, test.expectedRunError, errors.Cause(err))
			if test.expectedHookError == "" {
				if hookErr != nil {
					t.Fatal(hookErr)
				}
			} else {
				assert.Regexp(t, test.expectedHookError, hookErr.Error())
			}
			assert.Equal(t, test.expected, test.input)
		})
	}
}
