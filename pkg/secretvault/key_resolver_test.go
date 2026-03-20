package secretvault

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"
)

type inMemoryResolverSuite struct {
	suite.Suite
	defaultSecrets map[string]string
}

func TestInMemoryResolver(t *testing.T) {
	suite.Run(t, new(inMemoryResolverSuite))
}

func (s *inMemoryResolverSuite) SetupSuite() {
	s.defaultSecrets = make(map[string]string)
}

func (s *inMemoryResolverSuite) resolverWith(secrets map[string]string) KeyResolver {
	if secrets == nil {
		secrets = s.defaultSecrets
	}
	return newMapProvider(secrets)
}

func (s *inMemoryResolverSuite) TestGetSecretValue() {
	s.Run("returns value when key exists", func() {
		r := s.resolverWith(map[string]string{"aes-key": "base64key123", "api-key": "sk_live_xxx"})
		got, err := r.GetSecretValue(context.Background(), "aes-key")
		s.Require().NoError(err)
		s.Assert().Equal("base64key123", got)
		got, err = r.GetSecretValue(context.Background(), "api-key")
		s.Require().NoError(err)
		s.Assert().Equal("sk_live_xxx", got)
	})
	s.Run("returns error when name is empty", func() {
		r := s.resolverWith(map[string]string{"x": "y"})
		_, err := r.GetSecretValue(context.Background(), "")
		s.Require().Error(err)
	})
	s.Run("returns error when key missing", func() {
		r := s.resolverWith(map[string]string{"a": "b"})
		_, err := r.GetSecretValue(context.Background(), "missing")
		s.Require().Error(err)
	})
	s.Run("returns each value for multiple keys", func() {
		secrets := map[string]string{"k1": "v1", "k2": "v2", "k3": "v3"}
		r := s.resolverWith(secrets)
		for name, want := range secrets {
			got, err := r.GetSecretValue(context.Background(), name)
			s.Require().NoError(err)
			s.Assert().Equal(want, got)
		}
	})
	s.Run("returns empty string when secret value is empty", func() {
		r := s.resolverWith(map[string]string{"empty": ""})
		got, err := r.GetSecretValue(context.Background(), "empty")
		s.Require().NoError(err)
		s.Assert().Equal("", got)
	})
}

func (s *inMemoryResolverSuite) TestGetSecretValue_Concurrent() {
	s.Run("concurrent reads do not race", func() {
		r := s.resolverWith(map[string]string{"k1": "v1", "k2": "v2"})
		ctx := context.Background()
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					_, _ = r.GetSecretValue(ctx, "k1")
					_, _ = r.GetSecretValue(ctx, "k2")
				}
			}()
		}
		wg.Wait()
	})
	s.Run("concurrent read same key returns same value", func() {
		r := s.resolverWith(map[string]string{"x": "y"})
		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				v, _ := r.GetSecretValue(context.Background(), "x")
				s.Assert().Equal("y", v)
			}()
		}
		wg.Wait()
	})
	s.Run("concurrent read missing key returns error", func() {
		r := s.resolverWith(map[string]string{"a": "b"})
		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := r.GetSecretValue(context.Background(), "missing")
				s.Require().Error(err)
			}()
		}
		wg.Wait()
	})
	s.Run("concurrent read empty name returns error", func() {
		r := s.resolverWith(map[string]string{"x": "y"})
		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := r.GetSecretValue(context.Background(), "")
				s.Require().Error(err)
			}()
		}
		wg.Wait()
	})
	s.Run("sequential reads remain consistent", func() {
		r := s.resolverWith(map[string]string{"k": "v"})
		for i := 0; i < 100; i++ {
			got, err := r.GetSecretValue(context.Background(), "k")
			s.Require().NoError(err)
			s.Assert().Equal("v", got)
		}
	})
}

// -- MapProvider is a map-backed test mock for KeyResolver. It is used to test the KeyResolver interface.
type mapProvider struct {
	mu      sync.RWMutex
	secrets map[string]string
}

func newMapProvider(secrets map[string]string) *mapProvider {
	if secrets == nil {
		secrets = make(map[string]string)
	}
	return &mapProvider{secrets: secrets}
}

func (m *mapProvider) GetSecretValue(_ context.Context, name string) (string, error) {
	if name == "" {
		return "", errors.New("secret name is empty")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.secrets[name]
	if !ok {
		return "", errors.New("secret not found")
	}
	return v, nil
}
