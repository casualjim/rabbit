package steps

import (
	"context"

	"github.com/casualjim/rabbit/tasks/steps/internal"
)

// Logger interface for use in steps
type Logger interface {
	Debugf(string, ...interface{})
	Infof(string, ...interface{})
	Errorf(string, ...interface{})
}

// SetLogger on the context for usage in the steps
func SetLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, internal.LoggerKey, logger)
}

// ContextLogger gets the logger from the context
func ContextLogger(ctx context.Context) Logger {
	return ctx.Value(internal.LoggerKey).(Logger)
}
