package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

type testSuiteHash struct {
	suite.Suite
}

func TestHash(t *testing.T) {
	suite.Run(t, new(testSuiteHash))
}

func (s *testSuiteHash) TestSHA256Salted() {
	table := []struct {
		name string
		salt string
		data string
		want string
	}{
		{"normal", "mysalt", "mydata", hexEncodeSHA256Salted("mysalt", "mydata")},
		{"empty salt", "", "data", hexEncodeSHA256Salted("", "data")},
		{"empty data", "salt", "", hexEncodeSHA256Salted("salt", "")},
		{"card number", "pay", "4111111111111111", hexEncodeSHA256Salted("pay", "4111111111111111")},
		{"unicode", "sål", "dåtå", hexEncodeSHA256Salted("sål", "dåtå")},
	}
	for _, tc := range table {
		s.Run(tc.name, func() {
			got := SHA256Salted(tc.salt, tc.data)
			s.Assert().Equal(tc.want, got)
		})
	}
}

func (s *testSuiteHash) TestSHA256Hex() {
	table := []struct {
		name string
		data string
		want string
	}{
		{"hello", "hello", hex.EncodeToString(sha256Sum256([]byte("hello")))},
		{"empty", "", hex.EncodeToString(sha256Sum256(nil))},
		{"card", "4111111111111111", hex.EncodeToString(sha256Sum256([]byte("4111111111111111")))},
		{"single char", "x", hex.EncodeToString(sha256Sum256([]byte("x")))},
		{"with newline", "a\nb", hex.EncodeToString(sha256Sum256([]byte("a\nb")))},
	}
	for _, tc := range table {
		s.Run(tc.name, func() {
			got := SHA256Hex(tc.data)
			s.Assert().Equal(tc.want, got)
		})
	}
}

func hexEncodeSHA256Salted(salt, data string) string {
	h := sha256.New()
	h.Write([]byte(salt))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func sha256Sum256(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}

func (s *testSuiteHash) TestSHA256Salted_Deterministic() {
	s.Run("when same salt and data then same hash", func() {
		salt, data := "s", "d"
		s.Assert().Equal(SHA256Salted(salt, data), SHA256Salted(salt, data))
	})
	s.Run("when called 5 times then all equal", func() {
		h1 := SHA256Salted("a", "b")
		for i := 0; i < 4; i++ {
			s.Assert().Equal(h1, SHA256Salted("a", "b"))
		}
	})
	s.Run("when salt and data empty then deterministic", func() {
		s.Assert().Equal(SHA256Salted("", ""), SHA256Salted("", ""))
	})
	s.Run("when long inputs then deterministic", func() {
		salt := strings.Repeat("x", 1000)
		data := strings.Repeat("y", 1000)
		s.Assert().Equal(SHA256Salted(salt, data), SHA256Salted(salt, data))
	})
	s.Run("when unicode then deterministic", func() {
		s.Assert().Equal(SHA256Salted("日本", "日本語"), SHA256Salted("日本", "日本語"))
	})
}

func (s *testSuiteHash) TestSHA256Salted_OrderMatters() {
	s.Run("when salt and data order differs then hashes differ", func() {
		a, b := "a", "b"
		s.Assert().NotEqual(SHA256Salted(a, b), SHA256Salted(b, a))
	})
	s.Run("when salt and data swapped multiple pairs", func() {
		s.Assert().NotEqual(SHA256Salted("pay", "ref"), SHA256Salted("ref", "pay"))
		s.Assert().NotEqual(SHA256Salted("1", "2"), SHA256Salted("2", "1"))
	})
	s.Run("when prefix and suffix swapped", func() {
		s.Assert().NotEqual(SHA256Salted("pre", "suf"), SHA256Salted("suf", "pre"))
	})
	s.Run("when a and b swapped then different", func() {
		s.Assert().NotEqual(SHA256Salted("pay", "ref"), SHA256Salted("ref", "pay"))
	})
	s.Run("when long strings swapped", func() {
		a, b := "aaa", "bbb"
		s.Assert().NotEqual(SHA256Salted(a, b), SHA256Salted(b, a))
	})
}

func (s *testSuiteHash) TestSHA256Salted_ReturnsLowercaseHex() {
	s.Run("when input has unicode then output is 64 lowercase hex", func() {
		got := SHA256Salted("salt", "data")
		s.Require().Len(got, 64)
		s.Assert().Equal(got, strings.ToLower(got))
	})
	s.Run("when empty inputs then 64 hex", func() {
		got := SHA256Salted("", "")
		s.Require().Len(got, 64)
		s.Assert().Equal(got, strings.ToLower(got))
	})
	s.Run("when numeric then only 0-9a-f", func() {
		got := SHA256Salted("1", "2")
		for _, c := range got {
			s.Assert().True((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'))
		}
	})
	s.Run("when long salt then 64 hex", func() {
		got := SHA256Salted(strings.Repeat("x", 500), "d")
		s.Require().Len(got, 64)
	})
	s.Run("when special chars then 64 hex", func() {
		got := SHA256Salted("!@#", "$%^")
		s.Require().Len(got, 64)
	})
}

func (s *testSuiteHash) TestSHA256Hex_Deterministic() {
	s.Run("when same data then same hash", func() {
		data := "fintech"
		s.Assert().Equal(SHA256Hex(data), SHA256Hex(data))
	})
	s.Run("when empty then same", func() {
		s.Assert().Equal(SHA256Hex(""), SHA256Hex(""))
	})
	s.Run("when called 5 times then all equal", func() {
		h := SHA256Hex("x")
		for i := 0; i < 4; i++ {
			s.Assert().Equal(h, SHA256Hex("x"))
		}
	})
	s.Run("when unicode then deterministic", func() {
		s.Assert().Equal(SHA256Hex("日本"), SHA256Hex("日本"))
	})
	s.Run("when binary data string then deterministic", func() {
		data := string([]byte{0, 1, 2, 0xff})
		s.Assert().Equal(SHA256Hex(data), SHA256Hex(data))
	})
}

func (s *testSuiteHash) TestSHA256Hex_ReturnsLowercaseHex() {
	s.Run("when data is non-empty then 64 char lowercase hex", func() {
		got := SHA256Hex("x")
		s.Require().Len(got, 64)
		s.Assert().Equal(got, strings.ToLower(got))
	})
	s.Run("when data empty then 64 hex", func() {
		got := SHA256Hex("")
		s.Require().Len(got, 64)
		s.Assert().Equal(got, strings.ToLower(got))
	})
	s.Run("when data long then 64 hex", func() {
		got := SHA256Hex(strings.Repeat("a", 10000))
		s.Require().Len(got, 64)
	})
	s.Run("when data has spaces then 64 hex", func() {
		got := SHA256Hex("hello world")
		s.Require().Len(got, 64)
		s.Assert().Equal(got, strings.ToLower(got))
	})
	s.Run("when data is single space then 64 hex", func() {
		got := SHA256Hex(" ")
		s.Require().Len(got, 64)
	})
}

func (s *testSuiteHash) TestSHA256Hex_LargeInput() {
	s.Run("when data is 1MB then still 64 char hex", func() {
		data := strings.Repeat("a", 1024*1024)
		got := SHA256Hex(data)
		s.Assert().Len(got, 64)
	})
	s.Run("when data is 10KB then 64 hex", func() {
		got := SHA256Hex(strings.Repeat("x", 10*1024))
		s.Assert().Len(got, 64)
	})
	s.Run("when data is 100KB then 64 hex", func() {
		got := SHA256Hex(strings.Repeat("y", 100*1024))
		s.Assert().Len(got, 64)
	})
	s.Run("when data is 2MB then 64 hex", func() {
		got := SHA256Hex(strings.Repeat("z", 2*1024*1024))
		s.Assert().Len(got, 64)
	})
	s.Run("when data is 4KB then 64 hex", func() {
		got := SHA256Hex(strings.Repeat("a", 4096))
		s.Assert().Len(got, 64)
	})
}

