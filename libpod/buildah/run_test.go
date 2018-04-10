package buildah

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/opencontainers/runtime-tools/generate"
)

func TestAddRlimits(t *testing.T) {
	tt := []struct {
		name   string
		ulimit []string
		test   func(error, *generate.Generator) error
	}{
		{
			name:   "empty ulimit",
			ulimit: []string{},
			test: func(e error, g *generate.Generator) error {
				return e
			},
		},
		{
			name:   "invalid ulimit argument",
			ulimit: []string{"bla"},
			test: func(e error, g *generate.Generator) error {
				if e == nil {
					return fmt.Errorf("expected to receive an error but got nil")
				}
				errMsg := "invalid ulimit argument"
				if !strings.Contains(e.Error(), errMsg) {
					return fmt.Errorf("expected error message to include %#v in %#v", errMsg, e.Error())
				}
				return nil
			},
		},
		{
			name:   "invalid ulimit type",
			ulimit: []string{"bla=hard"},
			test: func(e error, g *generate.Generator) error {
				if e == nil {
					return fmt.Errorf("expected to receive an error but got nil")
				}
				errMsg := "invalid ulimit type"
				if !strings.Contains(e.Error(), errMsg) {
					return fmt.Errorf("expected error message to include %#v in %#v", errMsg, e.Error())
				}
				return nil
			},
		},
		{
			name:   "valid ulimit",
			ulimit: []string{"fsize=1024:4096"},
			test: func(e error, g *generate.Generator) error {
				if e != nil {
					return e
				}
				rlimits := g.Spec().Process.Rlimits
				for _, rlimit := range rlimits {
					if rlimit.Type == "RLIMIT_FSIZE" {
						if rlimit.Hard != 4096 {
							return fmt.Errorf("expected spec to have %#v hard limit set to %v but got %v", rlimit.Type, 4096, rlimit.Hard)
						}
						if rlimit.Soft != 1024 {
							return fmt.Errorf("expected spec to have %#v hard limit set to %v but got %v", rlimit.Type, 1024, rlimit.Soft)
						}
						return nil
					}
				}
				return fmt.Errorf("expected spec to have RLIMIT_FSIZE")
			},
		},
	}

	for _, te := range tt {
		g := generate.New()
		err := addRlimits(te.ulimit, &g)
		if testErr := te.test(err, &g); testErr != nil {
			t.Errorf("test %#v failed: %v", te.name, testErr)
		}
	}
}

func TestAddHosts(t *testing.T) {
	tt := []struct {
		name     string
		hosts    []string
		expected string
	}{
		{
			name:     "empty host list",
			hosts:    []string{},
			expected: "",
		},
		{
			name:     "host list",
			hosts:    []string{"localhost", "host-1", "host-2"},
			expected: "localhost\nhost-1\nhost-2\n",
		},
	}
	for _, te := range tt {
		buf := bytes.NewBufferString("")
		if err := addHosts(te.hosts, buf); err != nil {
			t.Errorf("expected test %#v to not receive error: %#v", te.name, err)
		}
		if buf.String() != te.expected {
			t.Errorf("test %#v failed: expected buffer to have %#v but got %#v",
				te.name, te.expected, buf.String())
		}
	}
}

func TestAddHostsToFile(t *testing.T) {
	tt := []struct {
		name     string
		prepare  func() (*os.File, error)
		hosts    []string
		expected string
	}{
		{
			name: "empty hosts file",
			prepare: func() (*os.File, error) {
				return ioutil.TempFile("", "buildah-hosts-test")
			},
			hosts:    []string{"host-1", "system-1"},
			expected: "host-1\nsystem-1\n",
		},
		{
			name: "append hosts to file",
			prepare: func() (*os.File, error) {
				file, err := ioutil.TempFile("", "buildah-hosts-test")
				if err != nil {
					return file, err
				}
				if _, err := file.WriteString("localhost\n"); err != nil {
					return file, fmt.Errorf("failed preparing file %#v to the test: %v", file.Name(), err)
				}
				if err := file.Close(); err != nil {
					return file, fmt.Errorf("failed preparing file %#v to the test: %v", file.Name(), err)
				}
				return file, nil
			},
			hosts:    []string{"host-0", "host-1", "dns.name.1"},
			expected: "localhost\nhost-0\nhost-1\ndns.name.1\n",
		},
		{
			name: "empty host list",
			prepare: func() (*os.File, error) {
				return ioutil.TempFile("", "buildah-hosts-test")
			},
			hosts:    []string{},
			expected: "",
		},
	}
	for _, te := range tt {
		file, err := te.prepare()
		if err != nil {
			t.Errorf("%#v", err.Error())
			if file != nil {
				os.Remove(file.Name())
			}
		}
		defer os.Remove(file.Name())
		if err := addHostsToFile(te.hosts, file.Name()); err != nil {
			t.Errorf("failed to add hosts to file: %v", err)
		}
		contentByte, err := ioutil.ReadFile(file.Name())
		if err != nil {
			t.Errorf("failed reading file %#v: %v", file.Name(), err)
		}
		res := string(contentByte)
		if res != te.expected {
			t.Errorf("test %#v failed: expected content to be %#v but got %#v", te.name, te.expected, res)
		}

	}
}

func TestAddHostsToNotExistFile(t *testing.T) {
	err := addHostsToFile([]string{"myhost"}, "/tmp/blabladjsghkg")
	if err == nil {
		t.Errorf("expected test to fail")
	}
	if !os.IsNotExist(err) {
		t.Errorf("expected error to fail because of 'no such file or directory' but got %#v", err.Error())
	}
}
