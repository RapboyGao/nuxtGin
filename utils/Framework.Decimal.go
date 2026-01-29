package utils

import "github.com/shopspring/decimal"

// ParseDecimal parses a decimal string.
func ParseDecimal(value string) (decimal.Decimal, error) {
	return decimal.NewFromString(value)
}

// ParseDecimalOrZero parses a decimal string and returns zero on error.
func ParseDecimalOrZero(value string) decimal.Decimal {
	result, err := decimal.NewFromString(value)
	if err != nil {
		return decimal.Zero
	}
	return result
}
