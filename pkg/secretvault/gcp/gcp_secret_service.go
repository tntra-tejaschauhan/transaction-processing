package gcp

import (
	"context"
	"errors"
	"fmt"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/api/option"
)

var (
	ErrProjectIDRequired = errors.New("gcp: projectID is required")
	ErrSecretNameEmpty   = errors.New("gcp: secret name must not be empty")
)

const secretVersionLatest = "latest"

// secretVersionAccessor is the GCP client subset used by GcpSecretService.
// Unexported so testutil can satisfy it via duck typing without an import cycle.
type secretVersionAccessor interface {
	AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest, opts ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error)
	Close() error
}

// GcpSecretService fetches secrets from Google Cloud Secret Manager.
type GcpSecretService struct {
	projectID string
	client    secretVersionAccessor
}

// NewGcpSecretService creates a GcpSecretService for the given GCP project.
// Call Close when done to release the client connection.
func NewGcpSecretService(ctx context.Context, projectID string, opts ...option.ClientOption) (*GcpSecretService, error) {
	if projectID == "" {
		return nil, ErrProjectIDRequired
	}
	client, err := secretmanager.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("gcp: failed to create secret manager client: %w", err)
	}
	return &GcpSecretService{projectID: projectID, client: client}, nil
}

func (g *GcpSecretService) secretResourceName(name string) string {
	return fmt.Sprintf("projects/%s/secrets/%s/versions/%s", g.projectID, name, secretVersionLatest)
}

func (g *GcpSecretService) GetSecretValue(ctx context.Context, name string) (string, error) {
	if name == "" {
		return "", ErrSecretNameEmpty
	}
	if g.client == nil {
		return "", errors.New("gcp: client is not initialised")
	}
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: g.secretResourceName(name),
	}
	resp, err := g.client.AccessSecretVersion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("gcp: %w", err)
	}
	if resp.Payload == nil || len(resp.Payload.Data) == 0 {
		return "", fmt.Errorf("gcp: secret %q returned an empty payload", name)
	}
	return string(resp.Payload.Data), nil
}

// Close releases the underlying client. Safe to call multiple times.
func (g *GcpSecretService) Close() error {
	if g.client != nil {
		return g.client.Close()
	}
	return nil
}
