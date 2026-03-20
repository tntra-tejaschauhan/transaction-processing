package secretvault

import "context"

// KeyResolver resolves a secret name to its value.
// Implementations must not include secret values in errors.
type KeyResolver interface {
	GetSecretValue(ctx context.Context, name string) (string, error)
}
