package is

import "errors"

var (
	ErrInvalidType        = errors.New("invalid type")
	ErrEmptySlice         = errors.New("slice is empty")
	ErrEmptyMap           = errors.New("map is empty")
	ErrEmptyString        = errors.New("string is empty")
	ErrNegativeInt        = errors.New("integer is negative")
	ErrNegativeFloat      = errors.New("float is negative")
	ErrMin                = errors.New("value is less than minimum")
	ErrMax                = errors.New("value is greater than maximum")
	ErrDate               = errors.New("date is invalid")
	ErrTimeZone           = errors.New("time zone is invalid")
	ErrRFC3339Time        = errors.New("time is not in RFC3339 format")
	ErrAlphaNumeric       = errors.New("string is not alphanumeric")
	ErrAlpha              = errors.New("string is not alphabetic")
	ErrInvalidEmailFormat = errors.New("email is invalid")
	ErrInvalidURLFormat   = errors.New("URL is invalid")
	ErrNotAlphanumeric    = errors.New("string is not alphanumeric")
	ErrNotAlpha           = errors.New("string is not alphabetic")
)
