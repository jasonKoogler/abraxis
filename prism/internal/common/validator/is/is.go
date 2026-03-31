package is

import (
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"time"
	"unicode"

	"github.com/jasonKoogler/prism/internal/common/util"
)

// String validators
func String(value string) error {
	if value == "" {
		return ErrEmptyString
	}
	return nil
}

func Email(value string) error {
	// Existing email validation logic
	var emailRegex = regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`)
	if !emailRegex.MatchString(value) {
		return ErrInvalidEmailFormat
	}
	return nil

}

func UUID(value string) error {
	// todo: consider return ErrInvalidUUIDFormat
	_, err := util.ParseUUID(value)
	if err != nil {
		return err
	}
	return nil
}

func URL(value string) error {
	_, err := url.Parse(value)
	if err != nil {
		return ErrInvalidURLFormat
	}
	return nil
}

func AlphaNumeric(value string) error {
	for _, char := range value {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')) {
			return ErrNotAlphanumeric
		}
	}
	return nil
}

func Alpha(value string) error {
	for _, char := range value {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')) {
			return ErrNotAlpha
		}
	}
	return nil
}

// Numeric validators
func NonNegativeInt(value int) error {
	if value < 0 {
		return ErrNegativeInt
	}
	return nil
}

func NonNegativeFloat32(value float32) error {
	if value < 0 {
		return ErrNegativeFloat
	}
	return nil
}

func Min(min int) func(int) error {
	return func(value int) error {
		if value < min {
			return fmt.Errorf("must be at least %d", min)
		}
		return nil
	}
}

func Max(max int) func(int) error {
	return func(value int) error {
		if value > max {
			return fmt.Errorf("must be at most %d", max)
		}
		return nil
	}
}

// Time-related validators
func Date(value string) error {
	_, err := time.Parse("2006-01-02", value)
	return err
}

func TimeZone(value string) error {
	_, err := time.LoadLocation(value)
	return err
}

func RFC3339Time(value string) error {
	_, err := time.Parse(time.RFC3339, value)
	return err
}

// Slice and map validators
func NonEmptySlice(value interface{}) error {
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Slice {
		return ErrInvalidType
	}
	if v.Len() == 0 {
		return ErrEmptySlice
	}
	return nil
}

func NonEmptyMap(value interface{}) error {
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Map {
		return ErrInvalidType
	}
	if v.Len() == 0 {
		return ErrEmptyMap
	}
	return nil
}

// Custom validators
func Length(min, max int) func(string) error {
	return func(value string) error {
		if len(value) < min || len(value) > max {
			return fmt.Errorf("length must be between %d and %d", min, max)
		}
		return nil
	}
}

func Pattern(pattern string) func(string) error {
	re := regexp.MustCompile(pattern)
	return func(value string) error {
		if !re.MatchString(value) {
			return fmt.Errorf("does not match pattern %s", pattern)
		}
		return nil
	}
}

// ValidPassword checks if the password is at least 8 characters long
func ValidPassword(value string) error {
	if len(value) < 8 {
		return fmt.Errorf("must be at least 8 characters")
	}

	// todo: add more checks
	hasUppercase := false
	hasLowercase := false
	hasDigit := false
	hasSpecial := false

	for _, char := range value {
		switch {
		case unicode.IsUpper(char):
			hasUppercase = true
		case unicode.IsLower(char):
			hasLowercase = true
		case unicode.IsDigit(char):
			hasDigit = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	if !hasUppercase {
		return fmt.Errorf("must contain at least one uppercase letter")
	}
	if !hasLowercase {
		return fmt.Errorf("must contain at least one lowercase letter")
	}
	if !hasDigit {
		return fmt.Errorf("must contain at least one digit")
	}
	if !hasSpecial {
		return fmt.Errorf("must contain at least one special character")
	}

	return nil
}

func ValidPhoneNumber(value string) error {

	// todo: maybe find a different solution as phonenumbers pkg uses reflection
	_, err := util.ValidateAndFormatPhoneNumber(value)
	if err != nil {
		return err
	}
	return nil
}

var hslRegex = regexp.MustCompile(`^hsl\(\s*(\d{1,3})\s*,\s*(\d{1,3})%\s*,\s*(\d{1,3})%\s*\)$`)

func HSLColor(value string) error {
	if !hslRegex.MatchString(value) {
		return fmt.Errorf("must be a valid HSL color string")
	}

	matches := hslRegex.FindStringSubmatch(value)
	if len(matches) != 4 {
		return fmt.Errorf("invalid HSL color format")
	}

	hue := parseInt(matches[1])
	saturation := parseInt(matches[2])
	lightness := parseInt(matches[3])

	if hue < 0 || hue > 360 {
		return fmt.Errorf("hue must be between 0 and 360")
	}
	if saturation < 0 || saturation > 100 {
		return fmt.Errorf("saturation must be between 0 and 100")
	}
	if lightness < 0 || lightness > 100 {
		return fmt.Errorf("lightness must be between 0 and 100")
	}

	return nil
}

func parseInt(s string) int {
	value, _ := strconv.Atoi(s)
	return value
}
