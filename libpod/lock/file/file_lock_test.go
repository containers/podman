package file

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test that creating and destroying locks work
func TestCreateAndDeallocate(t *testing.T) {
	d, err := ioutil.TempDir("", "filelock")
	assert.NoError(t, err)
	defer os.RemoveAll(d)

	_, err = OpenFileLock(filepath.Join(d, "locks"))
	assert.Error(t, err)

	l, err := CreateFileLock(filepath.Join(d, "locks"))
	assert.NoError(t, err)

	lock, err := l.AllocateLock()
	assert.NoError(t, err)

	err = l.AllocateGivenLock(lock)
	assert.Error(t, err)

	err = l.DeallocateLock(lock)
	assert.NoError(t, err)

	err = l.AllocateGivenLock(lock)
	assert.NoError(t, err)

	err = l.DeallocateAllLocks()
	assert.NoError(t, err)

	err = l.AllocateGivenLock(lock)
	assert.NoError(t, err)

	err = l.DeallocateAllLocks()
	assert.NoError(t, err)
}

// Test that creating and destroying locks work
func TestLockAndUnlock(t *testing.T) {
	d, err := ioutil.TempDir("", "filelock")
	assert.NoError(t, err)
	defer os.RemoveAll(d)

	l, err := CreateFileLock(filepath.Join(d, "locks"))
	assert.NoError(t, err)

	lock, err := l.AllocateLock()
	assert.NoError(t, err)

	err = l.LockFileLock(lock)
	assert.NoError(t, err)

	lslocks, err := exec.LookPath("lslocks")
	if err == nil {
		lockPath := l.getLockPath(lock)
		out, err := exec.Command(lslocks, "--json", "-p", fmt.Sprintf("%d", os.Getpid())).CombinedOutput()
		assert.NoError(t, err)

		assert.Contains(t, string(out), lockPath)
	}

	err = l.UnlockFileLock(lock)
	assert.NoError(t, err)
}
