package logging

import (
	"io"
	"os"
	"runtime"

	"github.com/mattn/go-colorable"
	"github.com/sirupsen/logrus"
)

// This is stdErrHook to send error to the stdErr.
type stdErrHook struct {
	stderr    io.Writer
	formatter logrus.Formatter
	level     logrus.Level
}

func newstdErrHook(level logrus.Level, formatter logrus.Formatter) *stdErrHook {
	// For windows to display colors we need to use the go-colorable writer
	if runtime.GOOS == "windows" {
		return &stdErrHook{
			stderr:    colorable.NewColorableStderr(),
			formatter: formatter,
			level:     level,
		}
	}
	return &stdErrHook{
		stderr:    os.Stderr,
		formatter: formatter,
		level:     level,
	}
}

func (h stdErrHook) Levels() []logrus.Level {
	var levels []logrus.Level
	for _, level := range logrus.AllLevels {
		if level <= h.level {
			levels = append(levels, level)
		}
	}
	return levels
}

func (h *stdErrHook) Fire(entry *logrus.Entry) error {
	line, err := h.formatter.Format(entry)
	if err != nil {
		return err
	}

	_, err = h.stderr.Write(line)
	return err
}
