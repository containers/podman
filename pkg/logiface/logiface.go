package logiface

type Logger interface {
	Errorf(format string, args ...interface{})
	Debugf(format string, args ...interface{})
}

var logger Logger

func SetLogger(l Logger) {
	logger = l
}

func Errorf(format string, args ...interface{}) {
	if logger != nil {
		logger.Errorf(format, args...)
	}
}

func Debugf(format string, args ...interface{}) {
	if logger != nil {
		logger.Debugf(format, args...)
	}
}
