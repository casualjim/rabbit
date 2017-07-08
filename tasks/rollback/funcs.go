package rollback

import "context"
import "github.com/hashicorp/errwrap"

// Always roll the execution back on any error
func Always(error) bool {
	return true
}

// Never roll the step back on an error
func Never(error) bool {
	return false
}

// OnCancel rolls the step back on cancel or timeout but not on errors
func OnCancel(err error) bool {
	errwrap.Contains(err, context.Canceled.Error())
	return err == context.Canceled || err == context.DeadlineExceeded
}
