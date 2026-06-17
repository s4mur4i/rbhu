package rbhu

import (
	"encoding/json"
	"fmt"
)

// APIError represents a non-2xx response from an RBHU endpoint.
type APIError struct {
	// StatusCode is the HTTP status code.
	StatusCode int
	// RequestID echoes the X-Request-ID header of the response, if present.
	RequestID string
	// Body is the raw response body.
	Body []byte

	// ErrorCode / ErrorDescription are parsed from the XS2A_Berlin_Error
	// envelope used by RBHU, when present.
	ErrorCode        string
	ErrorDescription string
	// TPPMessages holds Berlin Group tppMessages when the endpoint returns
	// that richer error shape instead.
	TPPMessages []TPPMessage
}

// TPPMessage is a single Berlin Group tppMessages entry.
type TPPMessage struct {
	Category string `json:"category"`
	Code     string `json:"code"`
	Path     string `json:"path,omitempty"`
	Text     string `json:"text,omitempty"`
}

func (e *APIError) Error() string {
	switch {
	case e.ErrorCode != "" || e.ErrorDescription != "":
		return fmt.Sprintf("rbhu: http %d: %s: %s", e.StatusCode, e.ErrorCode, e.ErrorDescription)
	case len(e.TPPMessages) > 0:
		m := e.TPPMessages[0]
		return fmt.Sprintf("rbhu: http %d: %s/%s: %s", e.StatusCode, m.Category, m.Code, m.Text)
	default:
		return fmt.Sprintf("rbhu: http %d: %s", e.StatusCode, string(e.Body))
	}
}

// parseAPIError builds an APIError from a response status, headers and body.
func parseAPIError(status int, requestID string, body []byte) *APIError {
	e := &APIError{StatusCode: status, RequestID: requestID, Body: body}

	var envelope struct {
		ErrorCode        string       `json:"errorCode"`
		ErrorDescription string       `json:"errorDescription"`
		TPPMessages      []TPPMessage `json:"tppMessages"`
	}
	if json.Unmarshal(body, &envelope) == nil {
		e.ErrorCode = envelope.ErrorCode
		e.ErrorDescription = envelope.ErrorDescription
		e.TPPMessages = envelope.TPPMessages
	}
	return e
}
