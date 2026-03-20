package apiresponse

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

type testSuiteAPIResponse struct {
	suite.Suite
}

func TestAPIResponse(t *testing.T) {
	suite.Run(t, new(testSuiteAPIResponse))
}

// ── Success ─────────────────────────────────────────────────────────────────

func (s *testSuiteAPIResponse) TestSuccess_Fields() {
	s.Run("when data is a map", func() {
		data := map[string]string{"key": "value"}
		env := Success(data, "corr-123")
		s.Assert().Equal(statusSuccess, env.Status)
		s.Assert().Equal(data, env.Data)
		s.Assert().Nil(env.Error)
		s.Assert().Equal("corr-123", env.CorrelationID)
	})
	s.Run("when data is nil", func() {
		env := Success(nil, "corr-456")
		s.Assert().Equal(statusSuccess, env.Status)
		s.Assert().Nil(env.Data)
		s.Assert().Nil(env.Error)
	})
	s.Run("when data is a primitive", func() {
		env := Success(42, "corr-789")
		s.Assert().Equal(42, env.Data)
	})
}

// ── Failure ──────────────────────────────────────────────────────────────────

func (s *testSuiteAPIResponse) TestFailure_Fields() {
	s.Run("when validation error", func() {
		apiErr := APIError{Code: ErrCodeValidation, Message: "invalid PAN"}
		env := Failure(apiErr, "corr-001")
		s.Assert().Equal(statusError, env.Status)
		s.Assert().Nil(env.Data)
		s.Require().NotNil(env.Error)
		s.Assert().Equal(ErrCodeValidation, env.Error.Code)
		s.Assert().Equal("invalid PAN", env.Error.Message)
		s.Assert().Equal("corr-001", env.CorrelationID)
	})
	s.Run("when internal error", func() {
		env := Failure(APIError{Code: ErrCodeInternal, Message: "unexpected"}, "corr-002")
		s.Assert().Equal(ErrCodeInternal, env.Error.Code)
	})
}

// ── JSON serialisation ───────────────────────────────────────────────────────

func (s *testSuiteAPIResponse) TestEnvelope_JSONShape() {
	s.Run("when success envelope serialises all fields", func() {
		env := Success(map[string]int{"amount": 100}, "corr-json")
		b, err := json.Marshal(env)
		s.Require().NoError(err)

		var raw map[string]any
		s.Require().NoError(json.Unmarshal(b, &raw))
		s.Assert().Equal("success", raw["status"])
		s.Assert().Nil(raw["error"])
		s.Assert().Equal("corr-json", raw["correlation_id"])
		s.Assert().NotNil(raw["data"])
	})
	s.Run("when failure envelope serialises error object", func() {
		env := Failure(APIError{Code: ErrCodeNotFound, Message: "resource missing"}, "corr-err")
		b, err := json.Marshal(env)
		s.Require().NoError(err)

		var raw map[string]any
		s.Require().NoError(json.Unmarshal(b, &raw))
		s.Assert().Equal("error", raw["status"])
		s.Assert().Nil(raw["data"])
		errObj, ok := raw["error"].(map[string]any)
		s.Require().True(ok)
		s.Assert().Equal(ErrCodeNotFound, errObj["code"])
		s.Assert().Equal("resource missing", errObj["message"])
	})
}

// ── Write ────────────────────────────────────────────────────────────────────

func (s *testSuiteAPIResponse) TestEnvelope_Write() {
	s.Run("when success write sets correct status code and content-type", func() {
		rec := httptest.NewRecorder()
		env := Success(map[string]string{"id": "txn-1"}, "corr-write")
		env.Write(rec, http.StatusOK)

		s.Assert().Equal(http.StatusOK, rec.Code)
		s.Assert().Equal("application/json", rec.Header().Get("Content-Type"))

		var decoded Envelope
		s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &decoded))
		s.Assert().Equal("success", decoded.Status)
		s.Assert().Equal("corr-write", decoded.CorrelationID)
	})
	s.Run("when failure write sets 422 and error body", func() {
		rec := httptest.NewRecorder()
		apiErr := APIError{Code: ErrCodeValidation, Message: "bad input"}
		Failure(apiErr, "corr-fail").Write(rec, http.StatusUnprocessableEntity)

		s.Assert().Equal(http.StatusUnprocessableEntity, rec.Code)
		var decoded Envelope
		s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &decoded))
		s.Assert().Equal("error", decoded.Status)
		s.Require().NotNil(decoded.Error)
		s.Assert().Equal(ErrCodeValidation, decoded.Error.Code)
	})
}

// ── Error codes ───────────────────────────────────────────────────────────────

func (s *testSuiteAPIResponse) TestErrorCodes_Defined() {
	s.Run("all sentinel codes are non-empty strings", func() {
		codes := []string{ErrCodeValidation, ErrCodeInternal, ErrCodeUnauthorized, ErrCodeNotFound}
		for _, c := range codes {
			s.Assert().NotEmpty(c)
		}
	})
}
