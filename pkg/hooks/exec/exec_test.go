package exec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
)

// path is the path to an example hook executable.
var path string

// unavoidableEnvironmentKeys may be injected even if the hook
// executable is executed with a requested empty environment.
var unavoidableEnvironmentKeys []string

func TestRun(t *testing.T) {
	ctx := context.Background()
	hook := &rspec.Hook{
		Path: path,
		Args: []string{"sh", "-c", "cat"},
	}
	var stderr, stdout bytes.Buffer
	hookErr, err := Run(ctx, hook, []byte("{}"), &stdout, &stderr, DefaultPostKillTimeout)
	if err != nil {
		t.Fatal(err)
	}
	if hookErr != nil {
		t.Fatal(hookErr)
	}
	assert.Equal(t, "{}", stdout.String())
	assert.Equal(t, "", stderr.String())
}

func TestRunIgnoreOutput(t *testing.T) {
	ctx := context.Background()
	hook := &rspec.Hook{
		Path: path,
		Args: []string{"sh", "-c", "cat"},
	}
	hookErr, err := Run(ctx, hook, []byte("{}"), nil, nil, DefaultPostKillTimeout)
	if err != nil {
		t.Fatal(err)
	}
	if hookErr != nil {
		t.Fatal(hookErr)
	}
}

func TestRunFailedStart(t *testing.T) {
	ctx := context.Background()
	hook := &rspec.Hook{
		Path: "/does/not/exist",
	}
	hookErr, err := Run(ctx, hook, []byte("{}"), nil, nil, DefaultPostKillTimeout)
	if err == nil {
		t.Fatal("unexpected success")
	}
	if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	assert.Equal(t, err, hookErr)
}

func parseEnvironment(input string) (env map[string]string, err error) {
	env = map[string]string{}
	lines := strings.Split(input, "\n")
	for i, line := range lines {
		if line == "" && i == len(lines)-1 {
			continue // no content after the terminal newline
		}
		keyValue := strings.SplitN(line, "=", 2)
		if len(keyValue) < 2 {
			return env, fmt.Errorf("no = in environment line: %q", line)
		}
		env[keyValue[0]] = keyValue[1]
	}
	for _, key := range unavoidableEnvironmentKeys {
		delete(env, key)
	}
	return env, nil
}

func TestRunEnvironment(t *testing.T) {
	ctx := context.Background()
	hook := &rspec.Hook{
		Path: path,
		Args: []string{"sh", "-c", "env"},
	}
	for _, tt := range []struct {
		name     string
		env      []string
		expected map[string]string
	}{
		{
			name:     "unset",
			expected: map[string]string{},
		},
		{
			name:     "set empty",
			env:      []string{},
			expected: map[string]string{},
		},
		{
			name: "set",
			env: []string{
				"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"TERM=xterm",
			},
			expected: map[string]string{
				"PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"TERM": "xterm",
			},
		},
	} {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			var stderr, stdout bytes.Buffer
			hook.Env = test.env
			hookErr, err := Run(ctx, hook, []byte("{}"), &stdout, &stderr, DefaultPostKillTimeout)
			if err != nil {
				t.Fatal(err)
			}
			if hookErr != nil {
				t.Fatal(hookErr)
			}
			assert.Equal(t, "", stderr.String())

			env, err := parseEnvironment(stdout.String())
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, test.expected, env)
		})
	}
}

func TestRunCancel(t *testing.T) {
	hook := &rspec.Hook{
		Path: path,
		Args: []string{"sh", "-c", "echo waiting; sleep 2; echo done"},
	}
	one := 1
	for _, tt := range []struct {
		name              string
		contextTimeout    time.Duration
		hookTimeout       *int
		expectedHookError string
		expectedRunError  error
		expectedStdout    string
	}{
		{
			name:           "no timeouts",
			expectedStdout: "waiting\ndone\n",
		},
		{
			name:              "context timeout",
			contextTimeout:    time.Duration(1) * time.Second,
			expectedStdout:    "waiting\n",
			expectedHookError: "^executing \\[sh -c echo waiting; sleep 2; echo done]: signal: killed$",
			expectedRunError:  context.DeadlineExceeded,
		},
		{
			name:              "hook timeout",
			hookTimeout:       &one,
			expectedStdout:    "waiting\n",
			expectedHookError: "^executing \\[sh -c echo waiting; sleep 2; echo done]: signal: killed$",
			expectedRunError:  context.DeadlineExceeded,
		},
	} {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			var stderr, stdout bytes.Buffer
			if test.contextTimeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, test.contextTimeout)
				defer cancel()
			}
			hook.Timeout = test.hookTimeout
			hookErr, err := Run(ctx, hook, []byte("{}"), &stdout, &stderr, DefaultPostKillTimeout)
			assert.Equal(t, test.expectedRunError, err)
			if test.expectedHookError == "" {
				if hookErr != nil {
					t.Fatal(hookErr)
				}
			} else {
				assert.Regexp(t, test.expectedHookError, hookErr.Error())
			}
			assert.Equal(t, "", stderr.String())
			assert.Equal(t, test.expectedStdout, stdout.String())
		})
	}
}

func TestRunKillTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(500)*time.Millisecond)
	defer cancel()
	hook := &rspec.Hook{
		Path: path,
		Args: []string{"sh", "-c", "sleep 1"},
	}
	hookErr, err := Run(ctx, hook, []byte("{}"), nil, nil, time.Duration(0))
	assert.Equal(t, context.DeadlineExceeded, err)
	assert.Regexp(t, "^(failed to reap process within 0s of the kill signal|executing \\[sh -c sleep 1]: signal: killed)$", hookErr)
}

func init() {
	if runtime.GOOS != "windows" {
		path = "/bin/sh"
		unavoidableEnvironmentKeys = []string{"PWD", "SHLVL", "_"}
	} else {
		panic("we need a reliable executable path on Windows")
	}
}
