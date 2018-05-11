package hooks

import (
	"context"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

// Monitor dynamically monitors hook directories for additions,
// updates, and removals.
//
// This function writes two empty structs to the sync channel: the
// first is written after the watchers are established and the second
// when this function exits.  The expected usage is:
//
//   ctx, cancel := context.WithCancel(context.Background())
//   sync := make(chan error, 2)
//   go m.Monitor(ctx, sync)
//   err := <-sync // block until writers are established
//   if err != nil {
//     return err // failed to establish watchers
//   }
//   // do stuff
//   cancel()
//   err = <-sync // block until monitor finishes
func (m *Manager) Monitor(ctx context.Context, sync chan<- error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		sync <- err
		return
	}
	defer watcher.Close()

	for _, dir := range m.directories {
		err = watcher.Add(dir)
		if err != nil {
			logrus.Errorf("failed to watch %q for hooks", dir)
			sync <- err
			return
		}
		logrus.Debugf("monitoring %q for hooks", dir)
	}

	sync <- nil

	for {
		select {
		case event := <-watcher.Events:
			filename := filepath.Base(event.Name)
			if len(m.directories) <= 1 {
				if event.Op&fsnotify.Remove == fsnotify.Remove {
					ok := m.remove(filename)
					if ok {
						logrus.Debugf("removed hook %s", event.Name)
					}
				} else if event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Write == fsnotify.Write {
					err = m.add(event.Name)
					if err == nil {
						logrus.Debugf("added hook %s", event.Name)
					} else if err != ErrNoJSONSuffix {
						logrus.Errorf("failed to add hook %s: %v", event.Name, err)
					}
				}
			} else if event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Remove == fsnotify.Remove {
				err = nil
				found := false
				for i := len(m.directories) - 1; i >= 0; i-- {
					path := filepath.Join(m.directories[i], filename)
					err = m.add(path)
					if err == nil {
						found = true
						logrus.Debugf("(re)added hook %s (triggered activity on %s)", path, event.Name)
						break
					} else if err == ErrNoJSONSuffix {
						found = true
						break // this is not going to change for fallback directories
					} else if os.IsNotExist(err) {
						continue // move on to the next fallback directory
					} else {
						found = true
						logrus.Errorf("failed to (re)add hook %s (triggered by activity on %s): %v", path, event.Name, err)
						break
					}
				}
				if (found || event.Op&fsnotify.Remove == fsnotify.Remove) && err != nil {
					ok := m.remove(filename)
					if ok {
						logrus.Debugf("removed hook %s (triggered by activity on %s)", filename, event.Name)
					}
				}
			}
		case <-ctx.Done():
			err = ctx.Err()
			logrus.Debugf("hook monitoring canceled: %v", err)
			sync <- err
			close(sync)
			return
		}
	}
}
