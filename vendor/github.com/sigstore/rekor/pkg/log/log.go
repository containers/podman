//
// Copyright 2021 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package log

import (
	"context"
	"log"

	"github.com/go-chi/chi/middleware"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger set the default logger to development mode
var Logger *zap.SugaredLogger

func init() {
	ConfigureLogger("dev")
}

func ConfigureLogger(logType string) {
	var cfg zap.Config
	if logType == "prod" {
		cfg = zap.NewProductionConfig()
		cfg.EncoderConfig.LevelKey = "severity"
		cfg.EncoderConfig.MessageKey = "message"
		cfg.EncoderConfig.TimeKey = "time"
		cfg.EncoderConfig.EncodeLevel = encodeLevel()
		cfg.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
		cfg.EncoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder
		cfg.EncoderConfig.EncodeCaller = zapcore.FullCallerEncoder
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	logger, err := cfg.Build()
	if err != nil {
		log.Fatalln("createLogger", err)
	}
	Logger = logger.Sugar()
}

func encodeLevel() zapcore.LevelEncoder {
	return func(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
		switch l {
		case zapcore.DebugLevel:
			enc.AppendString("DEBUG")
		case zapcore.InfoLevel:
			enc.AppendString("INFO")
		case zapcore.WarnLevel:
			enc.AppendString("WARNING")
		case zapcore.ErrorLevel:
			enc.AppendString("ERROR")
		case zapcore.DPanicLevel:
			enc.AppendString("CRITICAL")
		case zapcore.PanicLevel:
			enc.AppendString("ALERT")
		case zapcore.FatalLevel:
			enc.AppendString("EMERGENCY")
		}
	}
}

var CliLogger = createCliLogger()

func createCliLogger() *zap.SugaredLogger {
	cfg := zap.NewDevelopmentConfig()
	cfg.EncoderConfig.TimeKey = ""
	cfg.EncoderConfig.LevelKey = ""
	cfg.DisableCaller = true
	cfg.DisableStacktrace = true
	logger, err := cfg.Build()
	if err != nil {
		log.Fatalln("createLogger", err)
	}

	return logger.Sugar()
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, middleware.RequestIDKey, id)
}

type operation struct {
	id string
}

func (o operation) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("id", o.id)
	return nil
}

func ContextLogger(ctx context.Context) *zap.SugaredLogger {
	proposedLogger := Logger
	if ctx != nil {
		if ctxRequestID, ok := ctx.Value(middleware.RequestIDKey).(string); ok {
			requestID := operation{ctxRequestID}
			proposedLogger = proposedLogger.With(zap.Object("operation", requestID))
		}
	}
	return proposedLogger
}
