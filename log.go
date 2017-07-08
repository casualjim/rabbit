package rabbit

import (
	"context"
	"io"
	"log"
	"os"
)

// Logger is what your logrus-enabled library should take, that way
// it'll accept a stdlib logger and a logrus logger. There's no standard
// interface, this is the closest we get, unfortunately.
type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
}

// NopLogger represents a loger that throws everything to /dev/null
var NopLogger Logger

type noLog struct {
}

func (noLog) Debugf(str string, args ...interface{}) {
}

func (noLog) Infof(str string, args ...interface{}) {
}

func (noLog) Warnf(str string, args ...interface{}) {
}

func (noLog) Errorf(str string, args ...interface{}) {
}

func (noLog) Fatalf(str string, args ...interface{}) {
	os.Exit(1)
}

func init() {
	NopLogger = &noLog{}
}

// GoLog creates a step logger interface based on the standard logger
func GoLog(out io.Writer, prefix string, flag int) Logger {
	if out == nil {
		out = os.Stderr
	}
	return &stdlogger{
		debug: log.New(out, "[DEBUG] "+prefix, flag),
		info:  log.New(out, "[INFO]  "+prefix, flag),
		warn:  log.New(out, "[WARN]  "+prefix, flag),
		err:   log.New(out, "[ERROR] "+prefix, flag),
	}
}

type stdlogger struct {
	debug *log.Logger
	info  *log.Logger
	warn  *log.Logger
	err   *log.Logger
}

func (s *stdlogger) Debugf(str string, args ...interface{}) {
	s.debug.Printf(str, args...)
}

func (s *stdlogger) Infof(str string, args ...interface{}) {
	s.info.Printf(str, args...)
}

func (s *stdlogger) Warnf(str string, args ...interface{}) {
	s.warn.Printf(str, args...)
}

func (s *stdlogger) Errorf(str string, args ...interface{}) {
	s.err.Printf(str, args...)
}

func (s *stdlogger) Fatalf(str string, args ...interface{}) {
	s.err.Fatalf(str, args...)
}

type loggerCtxKey byte

const (
	loggerKey loggerCtxKey = 0
)

// SetLogger on the context for usage in the steps
func SetLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// ContextLogger gets the logger from the context
func ContextLogger(ctx context.Context) Logger {
	val := ctx.Value(loggerKey)
	if val == nil {
		return NopLogger
	}
	return val.(Logger)
}
