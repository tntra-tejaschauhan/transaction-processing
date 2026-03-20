package apiresponse

// APIError carries a machine-readable code and a human-readable message.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Standard error codes returned in the response envelope.
const (
	ErrCodeValidation   = "VALIDATION_ERROR"
	ErrCodeInternal     = "INTERNAL_ERROR"
	ErrCodeUnauthorized = "UNAUTHORIZED"
	ErrCodeNotFound     = "NOT_FOUND"
)
