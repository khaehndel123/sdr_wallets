package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	_default *zap.SugaredLogger
)

// configure a default logger
func init() {
	config := Config{
		DisableStacktrace: true,
	}

	ConfigureLogger(config)
}

type Config struct {
	Level             string   `mapstructure:"level"`
	Development       bool     `mapstructure:"development"`
	DisableStacktrace bool     `mapstructure:"disableStacktrace"`
	Encoding          string   `mapstructure:"encoding"`
	OutputPaths       []string `mapstructure:"outputPaths"`
	ErrorOutputPaths  []string `mapstructure:"errorOutputPaths"`
}

func (c *Config) applyDefaults() {
	if c.Level == "" {
		c.Level = "info"
	}
	if c.Encoding == "" {
		c.Encoding = "console"
	}
	if len(c.OutputPaths) == 0 {
		c.OutputPaths = []string{"stdout"}
	}
	if len(c.ErrorOutputPaths) == 0 {
		c.ErrorOutputPaths = []string{"stderr"}
	}
}

// ConfigureLogger configures a global zap logger instance.
func ConfigureLogger(c Config) *zap.SugaredLogger {
	c.applyDefaults()
	lvl := zapcore.InfoLevel
	_ = lvl.UnmarshalText([]byte(c.Level))
	logger, err := zap.Config{
		Level:       zap.NewAtomicLevelAt(lvl),
		Development: c.Development,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:          c.Encoding,
		EncoderConfig:     zap.NewDevelopmentEncoderConfig(),
		DisableStacktrace: c.DisableStacktrace,
		OutputPaths:       c.OutputPaths,
		ErrorOutputPaths:  c.ErrorOutputPaths,
	}.Build()
	logger = logger.WithOptions(zap.AddCallerSkip(1))

	if err != nil {
		panic(err)
	}

	_default = logger.Sugar()

	return _default
}

// Default returns the default global logger.
func Default() *zap.SugaredLogger {
	return _default
}

// Debug uses fmt.Sprint to construct and log a message.
func Debug(args ...interface{}) {
	Default().Debug(args...)
}

// Info uses fmt.Sprint to construct and log a message.
func Info(args ...interface{}) {
	Default().Info(args...)
}

// Warn uses fmt.Sprint to construct and log a message.
func Warn(args ...interface{}) {
	Default().Warn(args...)
}

// Error uses fmt.Sprint to construct and log a message.
func Error(args ...interface{}) {
	Default().Error(args...)
}

// Panic uses fmt.Sprint to construct and log a message, then panics.
func Panic(args ...interface{}) {
	Default().Panic(args...)
}

// Fatal uses fmt.Sprint to construct and log a message, then calls os.Exit.
func Fatal(args ...interface{}) {
	Default().Fatal(args...)
}

// Debugf uses fmt.Sprintf to log a templated message.
func Debugf(template string, args ...interface{}) {
	Default().Debugf(template, args...)
}

// Infof uses fmt.Sprintf to log a templated message.
func Infof(template string, args ...interface{}) {
	Default().Infof(template, args...)
}

// Warnf uses fmt.Sprintf to log a templated message.
func Warnf(template string, args ...interface{}) {
	Default().Warnf(template, args...)
}

// Errorf uses fmt.Sprintf to log a templated message.
func Errorf(template string, args ...interface{}) {
	Default().Errorf(template, args...)
}

// Panicf uses fmt.Sprintf to log a templated message, then panics.
func Panicf(template string, args ...interface{}) {
	Default().Panicf(template, args...)
}

// Fatalf uses fmt.Sprintf to log a templated message, then calls os.Exit.
func Fatalf(template string, args ...interface{}) {
	Default().Fatalf(template, args...)
}

// Debugw logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
//
// When debug-level logging is disabled, this is much faster than
//  s.With(keysAndValues).Debug(msg)
func Debugw(msg string, keysAndValues ...interface{}) {
	Default().Debugw(msg, keysAndValues...)
}

// Infow logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
func Infow(msg string, keysAndValues ...interface{}) {
	Default().Infow(msg, keysAndValues...)
}

// Warnw logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
func Warnw(msg string, keysAndValues ...interface{}) {
	Default().Warnw(msg, keysAndValues...)
}

// Errorw logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
func Errorw(msg string, keysAndValues ...interface{}) {
	Default().Errorw(msg, keysAndValues...)
}

// Panicw logs a message with some additional context, then panics. The
// variadic key-value pairs are treated as they are in With.
func Panicw(msg string, keysAndValues ...interface{}) {
	Default().Panicw(msg, keysAndValues...)
}

// Fatalw logs a message with some additional context, then calls os.Exit. The
// variadic key-value pairs are treated as they are in With.
func Fatalw(msg string, keysAndValues ...interface{}) {
	Default().Fatalw(msg, keysAndValues...)
}

// With adds a variadic number of fields to the logging context.
// It accepts a mix of strongly-typed zapcore.Field objects and loosely-typed key-value pairs.
func With(args ...interface{}) *zap.SugaredLogger {
	return Default().With(args...)
}
