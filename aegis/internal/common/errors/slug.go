package errors

type ErrorType struct {
	t string
}

var (
	ErrorTypeUnknown        = ErrorType{"unknown"}
	ErrorTypeAuthorization  = ErrorType{"authorization"}
	ErrorTypeIncorrectInput = ErrorType{"incorrect-input"}
)

type SlugError struct {
	err       string
	slug      string
	errorType ErrorType
}

func (s SlugError) Error() string {
	return s.err
}

func (s SlugError) Slug() string {
	return s.slug
}

func (s SlugError) ErrorType() ErrorType {
	return s.errorType
}

func NewSlugError(err string, slug string) SlugError {
	return SlugError{
		err:       err,
		slug:      slug,
		errorType: ErrorTypeUnknown,
	}
}

func NewAuthorizationError(err string, slug string) SlugError {
	return SlugError{
		err:       err,
		slug:      slug,
		errorType: ErrorTypeAuthorization,
	}
}

func NewIncorrectInputError(err string, slug string) SlugError {
	return SlugError{
		err:       err,
		slug:      slug,
		errorType: ErrorTypeIncorrectInput,
	}
}
