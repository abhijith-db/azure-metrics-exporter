package main

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger *zap.SugaredLogger
)

func initLogger() *zap.SugaredLogger {
	var config zap.Config
	if Opts.Logger.Development {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config = zap.NewProductionConfig()
	}

	config.Encoding = "console"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Set log level based on configuration
	switch Opts.Logger.Level {
	case "debug":
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	case "info":
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	case "warn":
		config.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	case "error":
		config.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	case "dpanic":
		config.Level = zap.NewAtomicLevelAt(zapcore.DPanicLevel)
	case "panic":
		config.Level = zap.NewAtomicLevelAt(zapcore.PanicLevel)
	case "fatal":
		config.Level = zap.NewAtomicLevelAt(zapcore.FatalLevel)
	default:
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}

	// debug level flag overrides log.level
	if Opts.Logger.Debug {
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	}

	// json log format
	if Opts.Logger.Json {
		config.Encoding = "json"

		// if running in containers, logs already enriched with timestamp by the container runtime
		config.EncoderConfig.TimeKey = ""
	}

	// build logger
	log, err := config.Build()
	if err != nil {
		panic(err)
	}

	logger = log.Sugar()

	return logger
}
