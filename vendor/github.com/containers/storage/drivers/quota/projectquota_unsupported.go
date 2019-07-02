// +build linux,exclude_disk_quota

package quota

import (
	"github.com/pkg/errors"
)

// Quota limit params - currently we only control blocks hard limit
type Quota struct {
	Size uint64
}

// Control - Context to be used by storage driver (e.g. overlay)
// who wants to apply project quotas to container dirs
type Control struct {
}

func NewControl(basePath string) (*Control, error) {
	return nil, errors.New("filesystem does not support, or has not enabled quotas")
}

// SetQuota - assign a unique project id to directory and set the quota limits
// for that project id
func (q *Control) SetQuota(targetPath string, quota Quota) error {
	return errors.New("filesystem does not support, or has not enabled quotas")
}

// GetQuota - get the quota limits of a directory that was configured with SetQuota
func (q *Control) GetQuota(targetPath string, quota *Quota) error {
	return errors.New("filesystem does not support, or has not enabled quotas")
}
