package cli

import (
	"testing"

	"github.com/mhiro2/envdesk/internal/crypto"
	"github.com/mhiro2/envdesk/internal/testutil/cryptotest"
)

func setupCryptoAdapter(t *testing.T, adapter crypto.Adapter) {
	t.Helper()
	original := newCryptoAdapter
	newCryptoAdapter = func() crypto.Adapter {
		return adapter
	}
	t.Cleanup(func() { newCryptoAdapter = original })
}

func setupPlaintextAdapter(t *testing.T) {
	t.Helper()
	setupCryptoAdapter(t, &cryptotest.PlaintextAdapter{})
}
