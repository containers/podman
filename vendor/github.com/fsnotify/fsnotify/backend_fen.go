//go:build solaris

// FEN backend for illumos (supported) and Solaris (untested, but should work).
//
// See port_create(3c) etc. for docs. https://www.illumos.org/man/3C/port_create

package fsnotify

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify/internal"
	"golang.org/x/sys/unix"
)

type fen struct {
	Events chan Event
	Errors chan error

	mu      sync.Mutex
	port    *unix.EventPort
	done    chan struct{} // Channel for sending a "quit message" to the reader goroutine
	dirs    map[string]Op // Explicitly watched directories
	watches map[string]Op // Explicitly watched non-directories
}

func newBackend(ev chan Event, errs chan error) (backend, error) {
	return newBufferedBackend(0, ev, errs)
}

func newBufferedBackend(sz uint, ev chan Event, errs chan error) (backend, error) {
	w := &fen{
		Events:  ev,
		Errors:  errs,
		dirs:    make(map[string]Op),
		watches: make(map[string]Op),
		done:    make(chan struct{}),
	}

	var err error
	w.port, err = unix.NewEventPort()
	if err != nil {
		return nil, fmt.Errorf("fsnotify.NewWatcher: %w", err)
	}

	go w.readEvents()
	return w, nil
}

// sendEvent attempts to send an event to the user, returning true if the event
// was put in the channel successfully and false if the watcher has been closed.
func (w *fen) sendEvent(name string, op Op) (sent bool) {
	select {
	case <-w.done:
		return false
	case w.Events <- Event{Name: name, Op: op}:
		return true
	}
}

// sendError attempts to send an error to the user, returning true if the error
// was put in the channel successfully and false if the watcher has been closed.
func (w *fen) sendError(err error) (sent bool) {
	if err == nil {
		return true
	}
	select {
	case <-w.done:
		return false
	case w.Errors <- err:
		return true
	}
}

func (w *fen) isClosed() bool {
	select {
	case <-w.done:
		return true
	default:
		return false
	}
}

func (w *fen) Close() error {
	// Take the lock used by associateFile to prevent lingering events from
	// being processed after the close
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.isClosed() {
		return nil
	}
	close(w.done)
	return w.port.Close()
}

func (w *fen) Add(name string) error { return w.AddWith(name) }

func (w *fen) AddWith(name string, opts ...addOpt) error {
	if w.isClosed() {
		return ErrClosed
	}
	if debug {
		fmt.Fprintf(os.Stderr, "FSNOTIFY_DEBUG: %s  AddWith(%q)\n",
			time.Now().Format("15:04:05.000000000"), name)
	}

	with := getOptions(opts...)
	if !w.xSupports(with.op) {
		return fmt.Errorf("%w: %s", xErrUnsupported, with.op)
	}

	// Currently we resolve symlinks that were explicitly requested to be
	// watched. Otherwise we would use LStat here.
	stat, err := os.Stat(name)
	if err != nil {
		return err
	}

	// Associate all files in the directory.
	if stat.IsDir() {
		err := w.handleDirectory(name, stat, true, w.associateFile)
		if err != nil {
			return err
		}

		w.mu.Lock()
		w.dirs[name] = with.op
		w.mu.Unlock()
		return nil
	}

	err = w.associateFile(name, stat, true)
	if err != nil {
		return err
	}

	w.mu.Lock()
	w.watches[name] = with.op
	w.mu.Unlock()
	return nil
}

func (w *fen) Remove(name string) error {
	if w.isClosed() {
		return nil
	}
	if !w.port.PathIsWatched(name) {
		return fmt.Errorf("%w: %s", ErrNonExistentWatch, name)
	}
	if debug {
		fmt.Fprintf(os.Stderr, "FSNOTIFY_DEBUG: %s  Remove(%q)\n",
			time.Now().Format("15:04:05.000000000"), name)
	}

	// The user has expressed an intent. Immediately remove this name from
	// whichever watch list it might be in. If it's not in there the delete
	// doesn't cause harm.
	w.mu.Lock()
	delete(w.watches, name)
	delete(w.dirs, name)
	w.mu.Unlock()

	stat, err := os.Stat(name)
	if err != nil {
		return err
	}

	// Remove associations for every file in the directory.
	if stat.IsDir() {
		err := w.handleDirectory(name, stat, false, w.dissociateFile)
		if err != nil {
			return err
		}
		return nil
	}

	err = w.port.DissociatePath(name)
	if err != nil {
		return err
	}

	return nil
}

// readEvents contains the main loop that runs in a goroutine watching for events.
func (w *fen) readEvents() {
	// If this function returns, the watcher has been closed and we can close
	// these channels
	defer func() {
		close(w.Errors)
		close(w.Events)
	}()

	pevents := make([]unix.PortEvent, 8)
	for {
		count, err := w.port.Get(pevents, 1, nil)
		if err != nil && err != unix.ETIME {
			// Interrupted system call (count should be 0) ignore and continue
			if errors.Is(err, unix.EINTR) && count == 0 {
				continue
			}
			// Get failed because we called w.Close()
			if errors.Is(err, unix.EBADF) && w.isClosed() {
				return
			}
			// There was an error not caused by calling w.Close()
			if !w.sendError(err) {
				return
			}
		}

		p := pevents[:count]
		for _, pevent := range p {
			if pevent.Source != unix.PORT_SOURCE_FILE {
				// Event from unexpected source received; should never happen.
				if !w.sendError(errors.New("Event from unexpected source received")) {
					return
				}
				continue
			}

			if debug {
				internal.Debug(pevent.Path, pevent.Events)
			}

			err = w.handleEvent(&pevent)
			if !w.sendError(err) {
				return
			}
		}
	}
}

func (w *fen) handleDirectory(path string, stat os.FileInfo, follow bool, handler func(string, os.FileInfo, bool) error) error {
	files, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	// Handle all children of the directory.
	for _, entry := range files {
		finfo, err := entry.Info()
		if err != nil {
			return err
		}
		err = handler(filepath.Join(path, finfo.Name()), finfo, false)
		if err != nil {
			return err
		}
	}

	// And finally handle the directory itself.
	return handler(path, stat, follow)
}

// handleEvent might need to emit more than one fsnotify event if the events
// bitmap matches more than one event type (e.g. the file was both modified and
// had the attributes changed between when the association was created and the
// when event was returned)
func (w *fen) handleEvent(event *unix.PortEvent) error {
	var (
		events     = event.Events
		path       = event.Path
		fmode      = event.Cookie.(os.FileMode)
		reRegister = true
	)

	w.mu.Lock()
	_, watchedDir := w.dirs[path]
	_, watchedPath := w.watches[path]
	w.mu.Unlock()
	isWatched := watchedDir || watchedPath

	if events&unix.FILE_DELETE != 0 {
		if !w.sendEvent(path, Remove) {
			return nil
		}
		reRegister = false
	}
	if events&unix.FILE_RENAME_FROM != 0 {
		if !w.sendEvent(path, Rename) {
			return nil
		}
		// Don't keep watching the new file name
		reRegister = false
	}
	if events&unix.FILE_RENAME_TO != 0 {
		// We don't report a Rename event for this case, because Rename events
		// are interpreted as referring to the _old_ name of the file, and in
		// this case the event would refer to the new name of the file. This
		// type of rename event is not supported by fsnotify.

		// inotify reports a Remove event in this case, so we simulate this
		// here.
		if !w.sendEvent(path, Remove) {
			return nil
		}
		// Don't keep watching the file that was removed
		reRegister = false
	}

	// The file is gone, nothing left to do.
	if !reRegister {
		if watchedDir {
			w.mu.Lock()
			delete(w.dirs, path)
			w.mu.Unlock()
		}
		if watchedPath {
			w.mu.Lock()
			delete(w.watches, path)
			w.mu.Unlock()
		}
		return nil
	}

	// If we didn't get a deletion the file still exists and we're going to have
	// to watch it again. Let's Stat it now so that we can compare permissions
	// and have what we need to continue watching the file

	stat, err := os.Lstat(path)
	if err != nil {
		// This is unexpected, but we should still emit an event. This happens
		// most often on "rm -r" of a subdirectory inside a watched directory We
		// get a modify event of something happening inside, but by the time we
		// get here, the sudirectory is already gone. Clearly we were watching
		// this path but now it is gone. Let's tell the user that it was
		// removed.
		if !w.sendEvent(path, Remove) {
			return nil
		}
		// Suppress extra write events on removed directories; they are not
		// informative and can be confusing.
		return nil
	}

	// resolve symlinks that were explicitly watched as we would have at Add()
	// time. this helps suppress spurious Chmod events on watched symlinks
	if isWatched {
		stat, err = os.Stat(path)
		if err != nil {
			// The symlink still exists, but the target is gone. Report the
			// Remove similar to above.
			if !w.sendEvent(path, Remove) {
				return nil
			}
			// Don't return the error
		}
	}

	if events&unix.FILE_MODIFIED != 0 {
		if fmode.IsDir() && watchedDir {
			if err := w.updateDirectory(path); err != nil {
				return err
			}
		} else {
			if !w.sendEvent(path, Write) {
				return nil
			}
		}
	}
	if events&unix.FILE_ATTRIB != 0 && stat != nil {
		// Only send Chmod if perms changed
		if stat.Mode().Perm() != fmode.Perm() {
			if !w.sendEvent(path, Chmod) {
				return nil
			}
		}
	}

	if stat != nil {
		// If we get here, it means we've hit an event above that requires us to
		// continue watching the file or directory
		return w.associateFile(path, stat, isWatched)
	}
	return nil
}

func (w *fen) updateDirectory(path string) error {
	// The directory was modified, so we must find unwatched entities and watch
	// them. If something was removed from the directory, nothing will happen,
	// as everything else should still be watched.
	files, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	for _, entry := range files {
		path := filepath.Join(path, entry.Name())
		if w.port.PathIsWatched(path) {
			continue
		}

		finfo, err := entry.Info()
		if err != nil {
			return err
		}
		err = w.associateFile(path, finfo, false)
		if !w.sendError(err) {
			return nil
		}
		if !w.sendEvent(path, Create) {
			return nil
		}
	}
	return nil
}

func (w *fen) associateFile(path string, stat os.FileInfo, follow bool) error {
	if w.isClosed() {
		return ErrClosed
	}
	// This is primarily protecting the call to AssociatePath but it is
	// important and intentional that the call to PathIsWatched is also
	// protected by this mutex. Without this mutex, AssociatePath has been seen
	// to error out that the path is already associated.
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.port.PathIsWatched(path) {
		// Remove the old association in favor of this one If we get ENOENT,
		// then while the x/sys/unix wrapper still thought that this path was
		// associated, the underlying event port did not. This call will have
		// cleared up that discrepancy. The most likely cause is that the event
		// has fired but we haven't processed it yet.
		err := w.port.DissociatePath(path)
		if err != nil && !errors.Is(err, unix.ENOENT) {
			return err
		}
	}

	var events int
	if !follow {
		// Watch symlinks themselves rather than their targets unless this entry
		// is explicitly watched.
		events |= unix.FILE_NOFOLLOW
	}
	if true { // TODO: implement withOps()
		events |= unix.FILE_MODIFIED
	}
	if true {
		events |= unix.FILE_ATTRIB
	}
	return w.port.AssociatePath(path, stat, events, stat.Mode())
}

func (w *fen) dissociateFile(path string, stat os.FileInfo, unused bool) error {
	if !w.port.PathIsWatched(path) {
		return nil
	}
	return w.port.DissociatePath(path)
}

func (w *fen) WatchList() []string {
	if w.isClosed() {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	entries := make([]string, 0, len(w.watches)+len(w.dirs))
	for pathname := range w.dirs {
		entries = append(entries, pathname)
	}
	for pathname := range w.watches {
		entries = append(entries, pathname)
	}

	return entries
}

func (w *fen) xSupports(op Op) bool {
	if op.Has(xUnportableOpen) || op.Has(xUnportableRead) ||
		op.Has(xUnportableCloseWrite) || op.Has(xUnportableCloseRead) {
		return false
	}
	return true
}
