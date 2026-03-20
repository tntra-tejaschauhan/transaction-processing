package validation

import (
	"errors"
	"strings"
)

const (
	amountMaxIntDigits = 12
	amountMaxDecimals  = 2
)

// ValidateAmount validates a monetary amount string. Rules:
//   - Must be numeric with an optional single decimal point
//   - Must be positive (non-zero)
//   - Integer part must not exceed 12 digits
//   - Decimal part must not exceed 2 digits
func ValidateAmount(amount string) error {
	if amount == "" {
		return errors.New("validate amount: must not be empty")
	}

	parts := strings.Split(amount, ".")
	if len(parts) > 2 {
		return errors.New("validate amount: invalid format, multiple decimal points")
	}

	intPart := parts[0]
	if intPart == "" {
		return errors.New("validate amount: integer part must not be empty")
	}
	for _, r := range intPart {
		if !isASCIIDigit(r) {
			return errors.New("validate amount: must contain digits only")
		}
	}
	if len(intPart) > amountMaxIntDigits {
		return errors.New("validate amount: integer part exceeds maximum of 12 digits")
	}

	if len(parts) == 2 {
		decPart := parts[1]
		if decPart == "" {
			return errors.New("validate amount: decimal part must not be empty when decimal point is present")
		}
		for _, r := range decPart {
			if !isASCIIDigit(r) {
				return errors.New("validate amount: decimal part must contain digits only")
			}
		}
		if len(decPart) > amountMaxDecimals {
			return errors.New("validate amount: decimal part exceeds maximum of 2 digits")
		}
	}

	// Must be positive (non-zero). Use the already-validated parts directly
	// to avoid allocating a new string via ReplaceAll.
	if strings.TrimLeft(intPart, "0") == "" &&
		(len(parts) < 2 || strings.TrimLeft(parts[1], "0") == "") {
		return errors.New("validate amount: must be greater than zero")
	}

	return nil
}
