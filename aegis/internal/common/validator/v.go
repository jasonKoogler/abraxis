package validator

import (
	"fmt"
	"strings"
)

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

type ValidationErrors []ValidationError

func (ve ValidationErrors) Error() string {
	if len(ve) == 0 {
		return ""
	}
	var errMsgs []string
	for _, err := range ve {
		errMsgs = append(errMsgs, err.Error())
	}
	return strings.Join(errMsgs, "; ")
}

type Validator struct {
	errors ValidationErrors
}

func New() *Validator {
	return &Validator{errors: ValidationErrors{}}
}

func (v *Validator) Errors() error {
	if len(v.errors) == 0 {
		return nil
	}
	return v.errors
}

func (v *Validator) AddError(field string, message string) {
	v.errors = append(v.errors, ValidationError{Field: field, Message: message})
}

func (v *Validator) Check(field string, value interface{}, checks ...func(interface{}) error) {
	for _, check := range checks {
		if err := check(value); err != nil {
			v.AddError(field, err.Error())
		}
	}
}

func (v *Validator) CheckString(field string, value string, checks ...func(string) error) {
	for _, check := range checks {
		if err := check(value); err != nil {
			v.AddError(field, err.Error())
		}
	}
}

func (v *Validator) CheckInt(field string, value int, checks ...func(int) error) {
	for _, check := range checks {
		if err := check(value); err != nil {
			v.AddError(field, err.Error())
		}
	}
}

func (v *Validator) CheckFloat32(field string, value float32, checks ...func(float32) error) {
	for _, check := range checks {
		if err := check(value); err != nil {
			v.AddError(field, err.Error())
		}
	}
}

func (v *Validator) CheckOptional(field string, value interface{}, checks ...func(interface{}) error) {
	if value == nil {
		return
	}
	v.Check(field, value, checks...)
}

func (v *Validator) CheckOptionalString(field string, value *string, checks ...func(string) error) {
	if value == nil {
		return
	}
	v.CheckString(field, *value, checks...)
}

func (v *Validator) CheckOptionalInt(field string, value *int, checks ...func(int) error) {
	if value == nil {
		return
	}
	v.CheckInt(field, *value, checks...)
}

func (v *Validator) CheckOptionalFloat32(field string, value *float32, checks ...func(float32) error) {
	if value == nil {
		return
	}
	v.CheckFloat32(field, *value, checks...)
}
