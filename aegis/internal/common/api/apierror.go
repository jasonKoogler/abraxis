package api

import (
	"fmt"
	"net/http"

	"github.com/go-chi/render"
	"github.com/jasonKoogler/abraxis/aegis/internal/common/log"
)

type ErrorType string

const (
	ErrorTypeInternal       ErrorType = "internal"
	ErrorTypeAuthorization  ErrorType = "authorization"
	ErrorTypeIncorrectInput ErrorType = "incorrect_input"
	ErrorTypeBadRequest     ErrorType = "bad_request"
	ErrorTypeNotFound       ErrorType = "not_found"
)

type APIError struct {
	Slug          string    `json:"slug" example:"invalid-json"`
	Message       string    `json:"message" example:"Invalid JSON request"`
	StatusCode    int       `json:"-"`
	ErrorType     ErrorType `json:"-"`
	internalError error
}

func (e APIError) Error() string {
	return fmt.Sprintf("API Error: %s (slug: %s)", e.Message, e.Slug)
}

func NewError(slug, message string, statusCode int, errorType ErrorType, internalError error) APIError {
	return APIError{
		Slug:          slug,
		Message:       message,
		StatusCode:    statusCode,
		ErrorType:     errorType,
		internalError: internalError,
	}
}

// Helper functions for common errors
func InternalError(err error) APIError {
	return NewError("internal_error", "An unexpected error occurred", http.StatusInternalServerError, ErrorTypeInternal, err)
}

func Unauthorized() APIError {
	return NewError("unauthorized", "Authentication required", http.StatusUnauthorized, ErrorTypeAuthorization, nil)
}

// NotFound is a convenience function for sending a user not found error
func NotFound(err error) APIError {
	return NewError("not-found", "Not found", http.StatusNotFound, ErrorTypeIncorrectInput, err)
}

// Conflict is a convenience function for sending a conflict error
func Conflict(slug, message string, err error) APIError {
	return NewError(slug, message, http.StatusConflict, ErrorTypeIncorrectInput, err)
}

func UserAlreadyExists(err error) APIError {
	return Conflict("user-already-exists", "User already exists", err)
}

//------ Bad Request Errors ------//

// These are errors that are caused by incorrect input from the user
func badRequest(slug, message string) APIError {
	return NewError(slug, message, http.StatusBadRequest, ErrorTypeIncorrectInput, nil)
}

// InvalidJSONError is a convenience function for sending an invalid JSON error
func InvalidJSONError() APIError {
	return badRequest("invalid-json", "Invalid JSON request")
}

// MissingIDError is a convenience function for sending a missing ID error
func MissingIDError() APIError {
	return badRequest("missing-id", "Missing ID")
}

// MissingFieldError is a convenience function for sending a missing field error
func MissingFieldError(field string) APIError {
	return badRequest("missing-field", "Missing field: "+field)
}

func InvalidQueryParamError(param string) APIError {
	return badRequest("invalid-query-param", "Invalid query parameter: "+param)
}

//------ End of Bad Request Errors ------///

// RespondWithError is a helper function for responding with an error
// It is used to send errors to the frontend
// It is also used to log errors to Sentry
func RespondWithError(err error, w http.ResponseWriter, r *http.Request) {
	apiErr, ok := err.(APIError)
	if !ok {
		apiErr = InternalError(err)
	}

	logEntry := log.GetLogEntry(r).With(
		log.String("slug", apiErr.Slug),
		log.Int64("status_code", int64(apiErr.StatusCode)),
	)

	if apiErr.internalError != nil {
		logEntry = logEntry.With(log.String("internal_error", apiErr.internalError.Error()))
	}

	logEntry.Warn(apiErr.Message)

	if renderErr := render.Render(w, r, apiErr); renderErr != nil {
		logEntry.Error("Failed to render error response", log.Error(renderErr))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Render satisfies the render.Renderer interface for go-chi/render
func (e APIError) Render(w http.ResponseWriter, r *http.Request) error {
	w.WriteHeader(e.StatusCode)

	return nil
}
