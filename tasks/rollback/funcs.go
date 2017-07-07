package rollback

import "context"

// Always roll the execution back on any error
func Always(error) bool {
	return true
}

// Abort never rolls the step back on an error
func Abort(error) bool {
	return false
}

// OnCancel rolls the step back on cancel or timeout but not on errors
func OnCancel(err error) bool {
	return err == context.Canceled || err == context.DeadlineExceeded
}
