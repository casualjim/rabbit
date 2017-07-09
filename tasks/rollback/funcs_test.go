package rollback

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFuncs(t *testing.T) {
	assert.True(t, Always(nil))
	assert.False(t, Never(nil))
	assert.False(t, OnCancel(assert.AnError))
	assert.True(t, OnCancel(context.Canceled))
}
