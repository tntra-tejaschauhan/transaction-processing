package apiresponse

import (
	"encoding/json"
	"net/http"
)

const (
	statusSuccess = "success"
	statusError   = "error"
)

// Envelope is the standard JSON response shape for all Go services.
//
//	{
//	  "status":         "success" | "error",
//	  "data":           <any> | null,
//	  "error":          null  | { "code": "...", "message": "..." },
//	  "correlation_id": "uuid-string"
//	}
type Envelope struct {
	Status        string     `json:"status"`
	Data          any        `json:"data"`
	Error         *APIError  `json:"error"`
	CorrelationID string     `json:"correlation_id"`
}

// Success returns an Envelope with status "success", the supplied data payload,
// and no error. correlationID should be the request-scoped trace identifier.
func Success(data any, correlationID string) Envelope {
	return Envelope{
		Status:        statusSuccess,
		Data:          data,
		Error:         nil,
		CorrelationID: correlationID,
	}
}

// Failure returns an Envelope with status "error", nil data, and the supplied
// APIError. correlationID should be the request-scoped trace identifier.
func Failure(apiErr APIError, correlationID string) Envelope {
	return Envelope{
		Status:        statusError,
		Data:          nil,
		Error:         &apiErr,
		CorrelationID: correlationID,
	}
}

// Write encodes the envelope as JSON, sets Content-Type to application/json,
// and writes the given HTTP status code. Any encoding error is silently ignored
// because the header has already been sent.
func (e Envelope) Write(w http.ResponseWriter, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(e)
}
