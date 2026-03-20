package appbase

import (
	"context"

	"github.com/samber/do"

	"github.com/PayWithSpireInc/transaction-processing/internal/crypto"
	"github.com/PayWithSpireInc/transaction-processing/pkg/secretvault"
	gcpSecretMgr "github.com/PayWithSpireInc/transaction-processing/pkg/secretvault/gcp"
)

// NewInjector builds a do.Injector pre-wired with all platform-level providers.
// Every provider is a lazy singleton — only initialised when first invoked.
func NewInjector(name string, cfg *Config) *do.Injector {
	i := do.New()

	//Config
	do.Provide(i, func(_ *do.Injector) (*Config, error) {
		return cfg, nil
	})

	//GCP Secret Manager client
	do.Provide(i, func(_ *do.Injector) (secretvault.KeyResolver, error) {
		return gcpSecretMgr.NewGcpSecretService(context.Background(), cfg.GCPProjectID)
	})

	// ===========================
	//	Crypto Module (HSM, Crypto Services)
	// ===========================

	do.Provide(i, func(j *do.Injector) (crypto.HSMCryptoService, error) {
		resolver, err := do.Invoke[secretvault.KeyResolver](j)
		if err != nil {
			return nil, err
		}
		return crypto.NewHSMCryptoService(resolver), nil
	})

	return i
}
