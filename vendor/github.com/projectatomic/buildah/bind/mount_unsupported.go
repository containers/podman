// +build !linux

package bind

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/mount"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// SetupIntermediateMountNamespace returns a no-op unmountAll() and no error.
func SetupIntermediateMountNamespace(spec *specs.Spec, bundlePath string) (unmountAll func() error, err error) {
	stripNoBuildahBindOption(spec)
	return func() error { return nil }, nil
}
