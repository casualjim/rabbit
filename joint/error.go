package joint

import "github.com/go-openapi/strfmt"

// Error the error model is a model for all the error responses coming from the lifecycle manager API
//
// swagger:model error
type Error struct {

	// cause
	Cause *Error `json:"cause,omitempty"`

	// The error code
	// Required: true
	Code int64 `json:"code"`

	// link to help page explaining the error in more detail
	HelpURL strfmt.URI `json:"helpUrl,omitempty"`

	// The error message
	// Required: true
	Message string `json:"message"`
}

func (m *Error) Error() string {
	if m.Cause != nil {
		return m.Message + ": " + m.Cause.Error()
	}
	return m.Message
}
