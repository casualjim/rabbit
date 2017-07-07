// Package tasks encapsulates a dsl for defining complex workflows.
//
// You define a task by giving a name and an entry point which is a step.
// Each task has a run and a rollback method.
package tasks

import "github.com/casualjim/rabbit/eventbus"

// A Task encapsulates the execution of a step and provides context and configuration.
// It serves as an executor and main interface for the user of the library.
// Steps assume they are being run by a task.
type Task interface {
	Run() error
	Rollback() error
	Close() error
	Subscribe(...eventbus.EventHandler)
	Unsubscribe(...eventbus.EventHandler)
}
