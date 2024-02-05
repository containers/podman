package logging

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	terminal "golang.org/x/term"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	lumberjackLogger *lumberjack.Logger
	logLevel         = defaultLogLevel()
	Memory           = newInMemoryHook(100)
)

func CloseLogging() {
	if lumberjackLogger != nil {
		_ = lumberjackLogger.Close()
	}
	logrus.StandardLogger().ReplaceHooks(make(logrus.LevelHooks))
}

func BackupLogFile() {
	if lumberjackLogger == nil {
		return
	}
	_ = lumberjackLogger.Rotate()
}

func InitLogrus(logFilePath string) {
	if lumberjackLogger != nil {
		return
	}

	lumberjackLogger = &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    5, // 5MB
		MaxBackups: 2,
	}
	// send logs to file
	logrus.SetOutput(lumberjackLogger)

	logrus.SetLevel(logrus.TraceLevel)

	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		level = logrus.InfoLevel
	}

	logrus.AddHook(Memory)

	// Add hook to send error/fatal to stderr
	logrus.AddHook(newstdErrHook(level, &logrus.TextFormatter{
		ForceColors:            terminal.IsTerminal(int(os.Stderr.Fd())),
		DisableTimestamp:       true,
		DisableLevelTruncation: false,
	}))
}

func DefaultLogLevel() logrus.Level {
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	return level
}

func defaultLogLevel() string {
	defaultLevel := "info"
	envLogLevel := os.Getenv("CRC_LOG_LEVEL")
	if envLogLevel != "" {
		defaultLevel = envLogLevel
	}

	return defaultLevel
}

func AddLogLevelFlag(flagset *pflag.FlagSet) {
	flagset.StringVar(&logLevel, "log-level", defaultLogLevel(), "log level (e.g. \"debug | info | warn | error\")")
}

func IsDebug() bool {
	return logLevel == "debug"
}

func Info(args ...interface{}) {
	logrus.Info(args...)
}

func Infof(s string, args ...interface{}) {
	logrus.Infof(s, args...)
}

func Warn(args ...interface{}) {
	logrus.Warn(args...)
}

func Warnf(s string, args ...interface{}) {
	logrus.Warnf(s, args...)
}

func Fatal(args ...interface{}) {
	logrus.Fatal(args...)
}

func Fatalf(s string, args ...interface{}) {
	logrus.Fatalf(s, args...)
}

func Error(args ...interface{}) {
	logrus.Error(args...)
}

func Errorf(s string, args ...interface{}) {
	logrus.Errorf(s, args...)
}

func Debug(args ...interface{}) {
	logrus.Debug(args...)
}

func Debugf(s string, args ...interface{}) {
	logrus.Debugf(s, args...)
}
