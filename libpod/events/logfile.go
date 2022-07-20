//go:build linux
// +build linux

package events

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/containers/podman/v4/pkg/util"
	"github.com/containers/storage/pkg/lockfile"
	"github.com/nxadm/tail"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// EventLogFile is the structure for event writing to a logfile. It contains the eventer
// options and the event itself.  Methods for reading and writing are also defined from it.
type EventLogFile struct {
	options EventerOptions
}

// Writes to the log file
func (e EventLogFile) Write(ee Event) error {
	// We need to lock events file
	lock, err := lockfile.GetLockfile(e.options.LogFilePath + ".lock")
	if err != nil {
		return err
	}
	lock.Lock()
	defer lock.Unlock()

	eventJSONString, err := ee.ToJSONString()
	if err != nil {
		return err
	}

	rotated, err := rotateLog(e.options.LogFilePath, eventJSONString, e.options.LogFileMaxSize)
	if err != nil {
		return fmt.Errorf("rotating log file: %w", err)
	}

	if rotated {
		rEvent := NewEvent(Rotate)
		rEvent.Type = System
		rEvent.Name = e.options.LogFilePath
		rotateJSONString, err := rEvent.ToJSONString()
		if err != nil {
			return err
		}
		if err := e.writeString(rotateJSONString); err != nil {
			return err
		}
	}

	return e.writeString(eventJSONString)
}

func (e EventLogFile) writeString(s string) error {
	f, err := os.OpenFile(e.options.LogFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0700)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(s + "\n"); err != nil {
		return err
	}
	return nil
}

func (e EventLogFile) getTail(options ReadOptions) (*tail.Tail, error) {
	reopen := true
	seek := tail.SeekInfo{Offset: 0, Whence: os.SEEK_END}
	if options.FromStart || !options.Stream {
		seek.Whence = 0
		reopen = false
	}
	stream := options.Stream
	return tail.TailFile(e.options.LogFilePath, tail.Config{ReOpen: reopen, Follow: stream, Location: &seek, Logger: tail.DiscardingLogger, Poll: true})
}

// Reads from the log file
func (e EventLogFile) Read(ctx context.Context, options ReadOptions) error {
	defer close(options.EventChannel)
	filterMap, err := generateEventFilters(options.Filters, options.Since, options.Until)
	if err != nil {
		return fmt.Errorf("failed to parse event filters: %w", err)
	}
	t, err := e.getTail(options)
	if err != nil {
		return err
	}
	if len(options.Until) > 0 {
		untilTime, err := util.ParseInputTime(options.Until, false)
		if err != nil {
			return err
		}
		go func() {
			time.Sleep(time.Until(untilTime))
			if err := t.Stop(); err != nil {
				logrus.Errorf("Stopping logger: %v", err)
			}
		}()
	}
	var line *tail.Line
	var ok bool
	for {
		select {
		case <-ctx.Done():
			// the consumer has cancelled
			t.Kill(errors.New("hangup by client"))
			return nil
		case line, ok = <-t.Lines:
			if !ok {
				// channel was closed
				return nil
			}
			// fallthrough
		}

		event, err := newEventFromJSONString(line.Text)
		if err != nil {
			return err
		}
		switch event.Type {
		case Image, Volume, Pod, System, Container, Network:
		//	no-op
		default:
			return fmt.Errorf("event type %s is not valid in %s", event.Type.String(), e.options.LogFilePath)
		}
		if applyFilters(event, filterMap) {
			options.EventChannel <- event
		}
	}
}

// String returns a string representation of the logger
func (e EventLogFile) String() string {
	return LogFile.String()
}

// Rotates the log file if the log file size and content exceeds limit
func rotateLog(logfile string, content string, limit uint64) (bool, error) {
	if limit == 0 {
		return false, nil
	}
	file, err := os.Stat(logfile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// The logfile does not exist yet.
			return false, nil
		}
		return false, err
	}
	var filesize = uint64(file.Size())
	var contentsize = uint64(len([]rune(content)))
	if filesize+contentsize < limit {
		return false, nil
	}

	if err := truncate(logfile); err != nil {
		return false, err
	}
	return true, nil
}

// Truncates the log file and saves 50% of content to new log file
func truncate(filePath string) error {
	orig, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer orig.Close()

	origFinfo, err := orig.Stat()
	if err != nil {
		return err
	}

	size := origFinfo.Size()
	threshold := size / 2

	tmp, err := ioutil.TempFile(path.Dir(filePath), "")
	if err != nil {
		// Retry in /tmp in case creating a tmp file in the same
		// directory has failed.
		tmp, err = ioutil.TempFile("", "")
		if err != nil {
			return err
		}
	}
	defer tmp.Close()

	// Jump directly to the threshold, drop the first line and copy the remainder
	if _, err := orig.Seek(threshold, 0); err != nil {
		return err
	}
	reader := bufio.NewReader(orig)
	if _, err := reader.ReadString('\n'); err != nil {
		if !errors.Is(err, io.EOF) {
			return err
		}
	}
	if _, err := reader.WriteTo(tmp); err != nil {
		return fmt.Errorf("writing truncated contents: %w", err)
	}

	if err := renameLog(tmp.Name(), filePath); err != nil {
		return fmt.Errorf("writing back %s to %s: %w", tmp.Name(), filePath, err)
	}

	return nil
}

// Renames from, to
func renameLog(from, to string) error {
	err := os.Rename(from, to)
	if err == nil {
		return nil
	}

	if !errors.Is(err, unix.EXDEV) {
		return err
	}

	// Files are not on the same partition, so we need to copy them the
	// hard way.
	fFrom, err := os.Open(from)
	if err != nil {
		return err
	}
	defer fFrom.Close()

	fTo, err := os.Create(to)
	if err != nil {
		return err
	}
	defer fTo.Close()

	if _, err := io.Copy(fTo, fFrom); err != nil {
		return fmt.Errorf("writing back from temporary file: %w", err)
	}

	if err := os.Remove(from); err != nil {
		return fmt.Errorf("removing temporary file: %w", err)
	}

	return nil
}
