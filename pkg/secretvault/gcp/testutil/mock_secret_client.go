// Package testutil provides test doubles for the gcp package.
package testutil

import (
	"context"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/googleapis/gax-go/v2"
)

// MockSecretClient is a controllable stand-in for the GCP Secret Manager client.
type MockSecretClient struct {
	Payload      []byte
	AccessErr    error
	EmptyPayload bool
	CloseErr     error
}

func (m *MockSecretClient) AccessSecretVersion(
	_ context.Context,
	_ *secretmanagerpb.AccessSecretVersionRequest,
	_ ...gax.CallOption,
) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	if m.AccessErr != nil {
		return nil, m.AccessErr
	}
	if m.EmptyPayload {
		return &secretmanagerpb.AccessSecretVersionResponse{
			Payload: &secretmanagerpb.SecretPayload{Data: nil},
		}, nil
	}
	return &secretmanagerpb.AccessSecretVersionResponse{
		Payload: &secretmanagerpb.SecretPayload{Data: m.Payload},
	}, nil
}

func (m *MockSecretClient) Close() error {
	return m.CloseErr
}
