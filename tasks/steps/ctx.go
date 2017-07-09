package steps

import (
	"context"
	"strings"

	"github.com/casualjim/rabbit/tasks/internal"
)

// StepPath reads the parent step name from the context and adds the local name as a dotted path to it
func StepPath(ctx context.Context, localName string) string {
	pn := GetParentName(ctx)
	if pn == "" {
		return localName
	}
	if localName == "" {
		return pn
	}
	return strings.Join([]string{pn, localName}, ".")
}

// SetParentName on the context
func SetParentName(ctx context.Context, name string) context.Context {
	if name == "" {
		return ctx
	}
	return context.WithValue(ctx, internal.ParentNameKey, name)
}

// GetParentName gets the step name from the context
func GetParentName(ctx context.Context) string {
	nm, ok := ctx.Value(internal.ParentNameKey).(string)
	if !ok {
		return ""
	}
	return nm
}

// SetStepName on the context
func SetStepName(ctx context.Context, name string) context.Context {
	if name == "" {
		return ctx
	}
	return context.WithValue(ctx, internal.StepNameKey, name)
}

// GetStepName gets the step name from the context
func GetStepName(ctx context.Context) string {
	nm, ok := ctx.Value(internal.StepNameKey).(string)
	if !ok {
		return ""
	}
	return nm
}

// SetTaskName on the context
func SetTaskName(ctx context.Context, name string) context.Context {
	if name == "" {
		return ctx
	}
	return context.WithValue(ctx, internal.TaskNameKey, name)
}

// GetTaskName gets the step name from the context
func GetTaskName(ctx context.Context) string {
	nm, ok := ctx.Value(internal.TaskNameKey).(string)
	if !ok {
		return ""
	}
	return nm
}
