package util

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	TimeFormatRFC3339 = time.RFC3339
)

func GenerateUUID() string {
	return uuid.New().String()
}

func Now() time.Time {
	return time.Now().UTC()
}

func ParseDate(date string) (*time.Time, error) {
	if date == "" {
		return nil, ErrDateEmpty
	}

	t, err := time.Parse(TimeFormatRFC3339, date)
	if err != nil {
		return nil, err
	}

	return &t, nil
}

func ParseDuration(duration string) (*time.Duration, error) {
	if duration == "" {
		return nil, ErrDurationEmpty
	}

	d, err := time.ParseDuration(duration)
	if err != nil {
		return nil, err
	}

	return &d, nil
}

func Int64ToDuration(i int64) *time.Duration {
	d := time.Duration(i) * time.Second
	return &d
}

// func ParseTime(t string) (*time.Time, error) {
// 	if t == "" {
// 		return nil, ErrTimeEmpty
// 	}

// 	parsedTime, err := time.Parse(TimeFormatRFC3339, t)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &parsedTime, nil
// }

func ParseTime(t string) (*time.Time, error) {
	if t == "" {
		return nil, ErrTimeEmpty
	}

	formats := []string{
		time.RFC3339,
		"2006-01-02 15:04:05 -0700 MST",
		// Add any other formats you might encounter
	}

	var parsedTime time.Time
	var err error
	for _, format := range formats {
		parsedTime, err = time.Parse(format, t)
		if err == nil {
			return &parsedTime, nil
		}
	}

	return nil, fmt.Errorf("unable to parse time string: %v", t)
}

func LoadLocation(location string) (*time.Location, error) {
	if location == "" {
		return nil, ErrLocationEmpty
	}

	loc, err := time.LoadLocation(location)
	if err != nil {
		return nil, err
	}

	return loc, nil
}

// type NullableTime struct {
// 	time.Time
// 	Valid bool
// }

// func NewNullableTime(t time.Time) *NullableTime {
// 	return &NullableTime{t, true}
// }

// Helper function to validate email (you might want to use a robust regex or a validation library)
func IsValidEmail(email string) bool {
	// Simple check for demonstration. Use a proper validation in real code.
	return strings.Contains(email, "@")
}

func IsValidUUID(id string) bool {
	_, err := uuid.Parse(id)
	return err == nil
}

func ParseUUID(id string) (uuid.UUID, error) {
	return uuid.Parse(id)
}

func ValidateAndFormatPhoneNumber(rawNumber string) (string, error) {
	// Remove any non-digit characters except '+'
	reg := regexp.MustCompile(`[^\d+]`)
	cleanNumber := reg.ReplaceAllString(rawNumber, "")

	// If number doesn't start with +, assume it's a US number
	if !strings.HasPrefix(cleanNumber, "+") {
		// If it starts with 1, just add the +
		if strings.HasPrefix(cleanNumber, "1") {
			cleanNumber = "+" + cleanNumber
		} else {
			// Otherwise, assume US and add +1
			cleanNumber = "+1" + cleanNumber
		}
	}

	// Validate the final format
	digits := cleanNumber[1:] // Remove the leading '+'
	if !regexp.MustCompile(`^\d+$`).MatchString(digits) {
		return "", ErrInvalidPhoneNumberFormat
	}

	// Check if the number has between 7 and 15 digits (excluding the '+')
	if len(digits) < 7 || len(digits) > 15 {
		return "", ErrInvalidPhoneNumberFormat
	}

	return cleanNumber, nil
}

func PtrString(s string) *string {
	return &s
}
func PtrUUID(u uuid.UUID) *uuid.UUID {
	return &u
}

func PtrTime(t time.Time) *time.Time {
	return &t
}

func PtrBool(b bool) *bool {
	return &b
}

func StringPtr(s string) *string {
	return &s
}

func IntPtr(i int) *int {
	return &i
}

func Int64Ptr(i int64) *int64 {
	return &i
}

func BoolPtr(b bool) *bool {
	return &b
}

func Float64Ptr(f float64) *float64 {
	return &f
}

func RetryWithExponentialBackoff(fn func() error, maxRetries int, initialDelay time.Duration) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		if i == maxRetries-1 {
			break
		}
		time.Sleep(initialDelay * time.Duration(1<<uint(i)))
	}
	return err
}

// DollarsToCents converts a float64 dollar amount to an integer cent amount.
// It rounds to the nearest cent.
func DollarsToCents(dollars float64) int {
	return int(math.Round(dollars * 100))
}

// PtrDollarsToCents converts a pointer to a float64 dollar amount to a pointer to an integer cent amount.
// It returns nil if the input is nil.
func PtrDollarsToCents(dollars *float64) *int {
	if dollars == nil {
		return nil
	}
	cents := DollarsToCents(*dollars)
	return &cents
}

func PtrStringSlice(s []string) *[]string {
	return &s
}

func PtrMapStringString(m map[string]string) *map[string]string {
	return &m
}

func PtrInt(i int) *int {
	return &i
}

func DefaultPageAndPageSize(page, pageSize *int) (*int, *int) {
	defaultPage := 1
	defaultPageSize := 10

	if page == nil {
		page = &defaultPage
	}
	if pageSize == nil {
		pageSize = &defaultPageSize
	}

	return page, pageSize
}

func HasUserPrefix(id string) bool {
	return strings.HasPrefix(id, "usr_")
}
func TrimUserPrefix(id string) (uuid.UUID, error) {
	if !HasUserPrefix(id) {
		return uuid.Nil, ErrInvalidUserPrefix
	}
	return uuid.Parse(strings.TrimPrefix(id, "usr_"))
}

func HasTenantPrefix(id string) bool {
	return strings.HasPrefix(id, "ten_")
}
func TrimTenantPrefix(id string) (uuid.UUID, error) {
	if !HasTenantPrefix(id) {
		return uuid.Nil, ErrInvalidTenantPrefix
	}
	return uuid.Parse(strings.TrimPrefix(id, "ten_"))
}

func HasSubscriptionPrefix(id string) bool {
	return strings.HasPrefix(id, "sub_")
}
func TrimSubscriptionPrefix(id string) (uuid.UUID, error) {
	if !HasSubscriptionPrefix(id) {
		return uuid.Nil, ErrInvalidSubscriptionPrefix
	}
	return uuid.Parse(strings.TrimPrefix(id, "sub_"))
}

func HasPlanPrefix(id string) bool {
	return strings.HasPrefix(id, "plan_")
}
func TrimPlanPrefix(id string) (uuid.UUID, error) {
	if !HasPlanPrefix(id) {
		return uuid.Nil, ErrInvalidPlanPrefix
	}
	return uuid.Parse(strings.TrimPrefix(id, "plan_"))
}

func HasPaymentMethodPrefix(id string) bool {
	return strings.HasPrefix(id, "paym_")
}
func TrimPaymentMethodPrefix(id string) (uuid.UUID, error) {
	if !HasPaymentMethodPrefix(id) {
		return uuid.Nil, ErrInvalidPaymentMethodPrefix
	}
	return uuid.Parse(strings.TrimPrefix(id, "paym_"))
}

func HasConnectedAccountPrefix(id string) bool {
	return strings.HasPrefix(id, "ca_")
}
func TrimConnectedAccountPrefix(id string) (uuid.UUID, error) {
	if !HasConnectedAccountPrefix(id) {
		return uuid.Nil, ErrInvalidConnectedAccountPrefix
	}
	return uuid.Parse(strings.TrimPrefix(id, "ca_"))
}

func HasCustomerPrefix(id string) bool {
	return strings.HasPrefix(id, "cus_")
}
func TrimCustomerPrefix(id string) (uuid.UUID, error) {
	if !HasCustomerPrefix(id) {
		return uuid.Nil, ErrInvalidCustomerPrefix
	}
	return uuid.Parse(strings.TrimPrefix(id, "cus_"))
}

func HasInvoicePrefix(id string) bool {
	return strings.HasPrefix(id, "inv_")
}
func TrimInvoicePrefix(id string) (uuid.UUID, error) {
	if !HasInvoicePrefix(id) {
		return uuid.Nil, ErrInvalidInvoicePrefix
	}
	return uuid.Parse(strings.TrimPrefix(id, "inv_"))
}

func HasOrganizationPrefix(id string) bool {
	return strings.HasPrefix(id, "org_")
}
func TrimOrganizationPrefix(id string) (uuid.UUID, error) {
	if !HasOrganizationPrefix(id) {
		return uuid.Nil, ErrInvalidOrganizationPrefix
	}
	return uuid.Parse(strings.TrimPrefix(id, "org_"))
}

var (
	ErrInvalidPhoneNumberFormat      = errors.New("invalid phone number format")
	ErrLocationNotFound              = errors.New("location not found")
	ErrDateEmpty                     = errors.New("date is empty")
	ErrDurationEmpty                 = errors.New("duration is empty")
	ErrTimeEmpty                     = errors.New("time is empty")
	ErrLocationEmpty                 = errors.New("location is empty")
	ErrInvalidUserPrefix             = errors.New("invalid user prefix")
	ErrInvalidTenantPrefix           = errors.New("invalid tenant prefix")
	ErrInvalidSubscriptionPrefix     = errors.New("invalid subscription prefix")
	ErrInvalidPlanPrefix             = errors.New("invalid plan prefix")
	ErrInvalidPaymentMethodPrefix    = errors.New("invalid payment method prefix")
	ErrInvalidConnectedAccountPrefix = errors.New("invalid connected account prefix")
	ErrInvalidCustomerPrefix         = errors.New("invalid customer prefix")
	ErrInvalidInvoicePrefix          = errors.New("invalid invoice prefix")
	ErrInvalidOrganizationPrefix     = errors.New("invalid organization prefix")
)
