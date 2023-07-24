package logpack

import (
	"sync"

	"go.uber.org/zap"
)

var global Logger
var gOnce sync.Once

func G() Logger {
	gOnce.Do(func() {
		var err error
		global, err = NewLogger("global")
		if err != nil {
			panic("failed to create global logger")
		}
	})
	return global
}

type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(fotmat string, args ...interface{})
}

type logger struct {
	sugar *zap.SugaredLogger
}

func NewLogger(selector string) (Logger, error) {
	
	log, err := zap.NewDevelopment()
	if err != nil {
		return nil, err
	}
	sl := &logger{
		sugar: log.Named(selector).Sugar(),
	}
	return sl, err
}

func (l *logger) Error(args ...interface{}) {
	l.sugar.Error(args...)
}

func (l *logger) Errorf(format string, args ...interface{}) {
	l.sugar.Errorf(format, args...)
}

func (l *logger) Debug(args ...interface{}) {
	l.sugar.Debug(args...)
}

func (l *logger) Info(args ...interface{}) {
	l.sugar.Info(args...)
}

func (l *logger) Debugf(format string, args ...interface{}) {
	l.sugar.Debugf(format, args...)
}

func (l *logger) Infof(format string, args ...interface{}) {
	l.sugar.Infof(format, args...)
}

/*
type Logger interface {
	// TODO
	// Core() zapcore.Core
	DPanic(args ...interface{})
	DPanicf(format string, args ...interface{})
	DPanicw(msg string, keysAndValues ...interface{})
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Debugw(msg string, keysAndValues ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Errorw(msg string, keysAndValues ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Fatalw(msg string, keysAndValues ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Infow(msg string, keysAndValues ...interface{})
	IsDebug() bool
	Named(name string) *logp.Logger
	Panic(args ...interface{})
	Panicf(format string, args ...interface{})
	Panicw(msg string, keysAndValues ...interface{})
	Recover(msg string)
	Sync() error
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Warnw(msg string, keysAndValues ...interface{})
	With(args ...interface{}) *logp.Logger
	WithOptions(options ...zap.Option) *logp.Logger
}

func Global() Logger {
	// TODO
}

type zapLogger struct {
	logger  *zap.Logger
	sugered *zap.SugaredLogger
}

type zapOption zap.Option

func newLogger(rootLogger *zap.Logger, selector string, options ...zapOption) *zapLogger {
	log := rootLogger.WithOptions(zap.AddCallerSkip(1)).
		WithOptions(options...). // zapOption must implement zap.Option interface
		Named(selector)
	return &zapLogger{log, log.Sugar()}
}

func NewZapLogger(selector string, options ...zapOption) *zapLogger {
	return newLogger(zap.New())
}*/
