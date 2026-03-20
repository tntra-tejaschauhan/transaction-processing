package validation

import "errors"

const (
	panMinLen = 13
	panMaxLen = 19
)

// ValidatePAN checks that pan is a valid payment card number: digits only,
// length 13–19, and passes the Luhn check.
func ValidatePAN(pan string) error {
	if len(pan) < panMinLen || len(pan) > panMaxLen {
		return errors.New("validate pan: length must be between 13 and 19 digits")
	}
	for _, r := range pan {
		if !isASCIIDigit(r) {
			return errors.New("validate pan: must contain digits only")
		}
	}
	if !LuhnCheck(pan) {
		return errors.New("validate pan: failed Luhn check")
	}
	return nil
}

// LuhnCheck returns true if the digit string passes the Luhn algorithm.
// Returns false immediately for any byte that is not an ASCII digit (0–9),
// which also guards against multi-byte UTF-8 sequences.
func LuhnCheck(digits string) bool {
	sum := 0
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		b := digits[i]
		if b < '0' || b > '9' {
			return false
		}
		d := int(b - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}
