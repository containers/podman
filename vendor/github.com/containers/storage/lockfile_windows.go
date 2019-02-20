// +build windows

package storage

import (
	"os"
	"sync"
	"time"
)

func getLockFile(path string, ro bool) (Locker, error) {
	return &lockfile{locked: false}, nil
}

type lockfile struct {
	mu     sync.Mutex
	file   string
	locked bool
}

func (l *lockfile) Lock() {
	l.mu.Lock()
	l.locked = true
}

func (l *lockfile) RLock() {
	l.mu.Lock()
	l.locked = true
}

func (l *lockfile) Unlock() {
	l.locked = false
	l.mu.Unlock()
}

func (l *lockfile) Locked() bool {
	return l.locked
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
