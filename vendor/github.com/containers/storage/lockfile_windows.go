// +build windows

package storage

import (
	"os"
	"sync"
	"time"
)

func getLockFile(path string, ro bool) (Locker, error) {
	return &lockfile{}, nil
}

type lockfile struct {
	mu   sync.Mutex
	file string
}

func (l *lockfile) Lock() {
}
func (l *lockfile) Unlock() {
}
func (l *lockfile) Modified() (bool, error) {
	return false, nil
}
func (l *lockfile) Touch() error {
	return nil
}
func (l *lockfile) IsReadWrite() bool {
	return false
}

func (l *lockfile) TouchedSince(when time.Time) bool {
	stat, err := os.Stat(l.file)
	if err != nil {
		return true
	}
	return when.Before(stat.ModTime())
}
