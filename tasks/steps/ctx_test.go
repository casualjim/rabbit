package steps_test

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/casualjim/rabbit/tasks/steps"
	"github.com/stretchr/testify/assert"
)

func TestConcurrent_ContextLogger(t *testing.T) {
	log := newLogger()
	ctx := steps.SetLogger(context.Background(), log)

	assert.Equal(t, log, steps.ContextLogger(ctx))

}

func newLogger() *stdlogger {
	return &stdlogger{
		debug: log.New(os.Stderr, "[DEBUG] ", 0),
		info:  log.New(os.Stderr, "[INFO] ", 0),
		err:   log.New(os.Stderr, "[ERROR] ", 0),
	}
}

type stdlogger struct {
	debug *log.Logger
	info  *log.Logger
	err   *log.Logger
}

func (s *stdlogger) Debugf(str string, args ...interface{}) {
	s.debug.Printf(str, args...)
}

func (s *stdlogger) Infof(str string, args ...interface{}) {
	s.debug.Printf(str, args...)
}

func (s *stdlogger) Errorf(str string, args ...interface{}) {
	s.debug.Printf(str, args...)
}
