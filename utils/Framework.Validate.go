package utils

import "github.com/go-playground/validator/v10"

// Validator returns a default validator instance.
func Validator() *validator.Validate {
	return validator.New()
}

// ValidateStruct validates a struct using a new validator instance.
func ValidateStruct(value interface{}) error {
	return Validator().Struct(value)
}

// ValidateVar validates a variable against a tag using a new validator instance.
func ValidateVar(value interface{}, tag string) error {
	return Validator().Var(value, tag)
}
