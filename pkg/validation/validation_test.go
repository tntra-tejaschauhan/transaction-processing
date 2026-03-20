package validation

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// ── Luhn / PAN ──────────────────────────────────────────────────────────────

type testSuiteValidation struct {
	suite.Suite
}

func TestValidation(t *testing.T) {
	suite.Run(t, new(testSuiteValidation))
}

func (s *testSuiteValidation) TestLuhnCheck() {
	table := []struct {
		name   string
		digits string
		want   bool
	}{
		{"valid Visa test PAN", "4111111111111111", true},
		{"valid Mastercard test PAN", "5500005555555559", true},
		{"valid Amex test PAN", "378282246310005", true},
		{"invalid – last digit wrong", "4111111111111112", false},
		{"all zeros", "0000000000000000", true},
		{"single zero", "0", true},
		{"single one", "1", false},
		{"non-ASCII digits return false", "411111111\u0661111111", false},
	}
	for _, tc := range table {
		s.Run(tc.name, func() {
			s.Assert().Equal(tc.want, LuhnCheck(tc.digits))
		})
	}
}

func (s *testSuiteValidation) TestValidatePAN_Valid() {
	s.Run("when valid Visa test PAN", func() {
		s.Require().NoError(ValidatePAN("4111111111111111"))
	})
	s.Run("when valid 13-digit PAN", func() {
		// 4222222222222 passes Luhn
		s.Require().NoError(ValidatePAN("4222222222222"))
	})
	s.Run("when valid 19-digit PAN", func() {
		// 6304900017240292446 – 19-digit Maestro test PAN, passes Luhn
		s.Require().NoError(ValidatePAN("6304900017240292446"))
	})
}

func (s *testSuiteValidation) TestValidatePAN_Invalid() {
	table := []struct {
		name string
		pan  string
	}{
		{"too short", "411111111111"},
		{"too long", "41111111111111111119"},
		{"contains letters", "411111111111111A"},
		{"contains spaces", "4111 1111 1111 1111"},
		{"fails Luhn", "4111111111111112"},
		{"empty string", ""},
		{"non-ASCII digits mixed with ASCII", "4111111111\u0661\u0661\u0661"},
	}
	for _, tc := range table {
		s.Run(tc.name, func() {
			s.Assert().Error(ValidatePAN(tc.pan))
		})
	}
}

// ── Amount ──────────────────────────────────────────────────────────────────

func (s *testSuiteValidation) TestValidateAmount_Valid() {
	table := []struct {
		name   string
		amount string
	}{
		{"integer only", "100"},
		{"two decimals", "9.99"},
		{"one decimal", "1.5"},
		{"large amount", "999999999999"},
		{"minimum positive", "0.01"},
		{"max int digits with decimals", "123456789012.99"},
	}
	for _, tc := range table {
		s.Run(tc.name, func() {
			s.Require().NoError(ValidateAmount(tc.amount))
		})
	}
}

func (s *testSuiteValidation) TestValidateAmount_Invalid() {
	table := []struct {
		name   string
		amount string
	}{
		{"empty string", ""},
		{"zero integer", "0"},
		{"zero decimal", "0.00"},
		{"negative sign", "-1.00"},
		{"letters", "abc"},
		{"multiple dots", "1.2.3"},
		{"trailing dot", "100."},
		{"leading dot", ".99"},
		{"too many decimals", "1.999"},
		{"integer exceeds 12 digits", "1234567890123"},
		{"non-ASCII digits in integer part", "\u0661\u0662\u0663"},
		{"non-ASCII digits in decimal part", "1.\u0662\u0663"},
	}
	for _, tc := range table {
		s.Run(tc.name, func() {
			s.Assert().Error(ValidateAmount(tc.amount))
		})
	}
}

// ── SanitizeString ──────────────────────────────────────────────────────────

func (s *testSuiteValidation) TestSanitizeString() {
	s.Run("when trims leading and trailing whitespace", func() {
		s.Assert().Equal("hello", SanitizeString("  hello  ", 0))
	})
	s.Run("when removes control characters", func() {
		s.Assert().Equal("hello world", SanitizeString("hello\x00 world\x01", 0))
	})
	s.Run("when truncates to maxLen", func() {
		s.Assert().Equal("hel", SanitizeString("hello", 3))
	})
	s.Run("when maxLen zero means no truncation", func() {
		s.Assert().Equal("hello world", SanitizeString("hello world", 0))
	})
	s.Run("when empty string returns empty", func() {
		s.Assert().Equal("", SanitizeString("", 10))
	})
	s.Run("when unicode string truncated correctly", func() {
		// "日本語" is 3 runes
		s.Assert().Equal("日本", SanitizeString("日本語", 2))
	})
	s.Run("when string shorter than maxLen is unchanged", func() {
		s.Assert().Equal("hi", SanitizeString("hi", 100))
	})
	s.Run("when tab is treated as control char", func() {
		s.Assert().Equal("ab", SanitizeString("a\tb", 0))
	})
}

// ── SanitizeSQL ──────────────────────────────────────────────────────────────

func (s *testSuiteValidation) TestSanitizeSQL() {
	s.Run("when clean string passes through unchanged", func() {
		s.Assert().Equal("John Doe", SanitizeSQL("John Doe", 0))
	})
	s.Run("when single quote is escaped", func() {
		s.Assert().Equal("O''Brien", SanitizeSQL("O'Brien", 0))
	})
	s.Run("when line comment is removed", func() {
		// single quote is escaped to '' and -- is stripped
		s.Assert().Equal("admin''", SanitizeSQL("admin'--", 0))
	})
	s.Run("when block comment markers are removed", func() {
		// /* and */ are stripped; content between them is left (not a full parser)
		s.Assert().Equal("1 OR 1comment1", SanitizeSQL("1 OR 1/*comment*/1", 0))
	})
	s.Run("when statement terminator is removed", func() {
		s.Assert().Equal("foo DROP TABLE users", SanitizeSQL("foo; DROP TABLE users", 0))
	})
	s.Run("when backslash is removed", func() {
		s.Assert().Equal("value", SanitizeSQL(`val\ue`, 0))
	})
	s.Run("when classic injection payload is neutralised", func() {
		// ' OR '1'='1  →  '' OR ''1''=''1  (quotes escaped, no runaway string)
		out := SanitizeSQL("' OR '1'='1", 0)
		s.Assert().NotContains(out, "--")
		s.Assert().NotContains(out, ";")
		// all single quotes must be doubled (none left unescaped)
		s.Assert().NotRegexp(`[^']'[^']`, out)
	})
	s.Run("when maxLen is respected after SQL sanitization", func() {
		s.Assert().Equal("hel", SanitizeSQL("hello", 3))
	})
	s.Run("when quote escaping expands string maxLen is still honoured", func() {
		s.Assert().Equal("a''", SanitizeSQL("a'b", 3))
	})
	s.Run("when whitespace is trimmed before SQL sanitization", func() {
		s.Assert().Equal("hello", SanitizeSQL("  hello  ", 0))
	})
}
