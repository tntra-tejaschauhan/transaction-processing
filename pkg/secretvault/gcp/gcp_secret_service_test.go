package gcp

import (
	"context"
	"errors"
	"testing"

	"github.com/PayWithSpireInc/transaction-processing/pkg/secretvault/gcp/testutil"
	"github.com/stretchr/testify/suite"
)

type gcpSecretServiceSuite struct {
	suite.Suite
	secretMgr  *GcpSecretService
	mockClient *testutil.MockSecretClient
}

func TestGcpSecretService(t *testing.T) {
	suite.Run(t, new(gcpSecretServiceSuite))
}

func (s *gcpSecretServiceSuite) SetupSubTest() {
	s.mockClient = &testutil.MockSecretClient{}
	s.secretMgr = &GcpSecretService{
		projectID: "test-project",
		client:    s.mockClient,
	}
}

func (s *gcpSecretServiceSuite) TestNewGcpSecretService() {
	s.Run("returns sentinel error and nil when projectID is empty", func() {
		svc, err := NewGcpSecretService(context.Background(), "")
		s.Require().ErrorIs(err, ErrProjectIDRequired)
		s.Require().Nil(svc)
	})
}

func (s *gcpSecretServiceSuite) TestGetSecretValue() {
	s.Run("returns secret value on success", func() {
		s.mockClient.Payload = []byte("super-secret-value")
		got, err := s.secretMgr.GetSecretValue(context.Background(), "my-secret")
		s.Require().NoError(err)
		s.Assert().Equal("super-secret-value", got)
	})
	s.Run("returns sentinel error when name is empty", func() {
		val, err := s.secretMgr.GetSecretValue(context.Background(), "")
		s.Require().ErrorIs(err, ErrSecretNameEmpty)
		s.Assert().Empty(val)
	})
	s.Run("returns error when client is not initialised", func() {
		s.secretMgr.client = nil
		_, err := s.secretMgr.GetSecretValue(context.Background(), "some-secret")
		s.Require().Error(err)
		s.Assert().Contains(err.Error(), "not initialised")
	})
	s.Run("wraps error returned by GCP client", func() {
		s.mockClient.AccessErr = errors.New("permission denied")
		_, err := s.secretMgr.GetSecretValue(context.Background(), "some-secret")
		s.Require().Error(err)
		s.Assert().Contains(err.Error(), "permission denied")
	})
	s.Run("returns error when payload is nil", func() {
		s.mockClient.EmptyPayload = true
		_, err := s.secretMgr.GetSecretValue(context.Background(), "some-secret")
		s.Require().Error(err)
		s.Assert().Contains(err.Error(), "empty payload")
	})
}

func (s *gcpSecretServiceSuite) TestClose() {
	s.Run("no error when client closes successfully", func() {
		s.Require().NoError(s.secretMgr.Close())
	})
	s.Run("propagates error returned by client Close", func() {
		s.mockClient.CloseErr = errors.New("close failed")
		s.Require().EqualError(s.secretMgr.Close(), "close failed")
	})
	s.Run("no error when client is nil", func() {
		s.secretMgr.client = nil
		s.Require().NoError(s.secretMgr.Close())
	})
	s.Run("no error on zero-value struct", func() {
		var zero GcpSecretService
		s.Require().NoError(zero.Close())
	})
}

func (s *gcpSecretServiceSuite) TestImplementsProvider() {
		s.Run("GcpSecretService is assignable to secretvault.KeyResolver", func() {
		type providerShape interface {
			GetSecretValue(ctx context.Context, name string) (string, error)
		}
		var p providerShape = s.secretMgr
		s.Require().NotNil(p)
	})
	s.Run("nil *GcpSecretService still carries the type information", func() {
		var nilSvc *GcpSecretService
		type providerShape interface {
			GetSecretValue(ctx context.Context, name string) (string, error)
		}
		var p providerShape = nilSvc
		_, ok := p.(*GcpSecretService)
		s.Require().True(ok)
	})
}
