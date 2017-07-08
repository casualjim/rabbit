package rabbit_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/casualjim/rabbit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextLogger(t *testing.T) {
	log := rabbit.GoLog(nil, "", 0)
	ctx := rabbit.SetLogger(context.Background(), log)

	require.Equal(t, log, rabbit.ContextLogger(ctx))

	var buf bytes.Buffer
	log = rabbit.GoLog(&buf, "", 0)

	log.Debugf("level")
	log.Infof("level")
	log.Warnf("level")
	log.Errorf("level")

	str := buf.String()
	assert.Contains(t, str, "[DEBUG] level")
	assert.Contains(t, str, "[INFO]  level")
	assert.Contains(t, str, "[WARN]  level")
	assert.Contains(t, str, "[ERROR] level")

	log = rabbit.ContextLogger(context.Background())
	assert.Equal(t, rabbit.NopLogger, log)
	log.Debugf("level")
	log.Infof("level")
	log.Warnf("level")
	log.Errorf("level")

}

func TestNopLoggerFatal(t *testing.T) {

	if os.Getenv("LOG_FATAL_TEST") == "1" {
		rabbit.NopLogger.Fatalf("level")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestNopLoggerFatal$")
	cmd.Env = append(os.Environ(), "LOG_FATAL_TEST=1")
	err := cmd.Run()
	require.IsType(t, &exec.ExitError{}, err)
	require.False(t, err.(*exec.ExitError).Success())
}
