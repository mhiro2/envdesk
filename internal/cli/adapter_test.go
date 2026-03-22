package cli

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/mhiro2/envdesk/internal/crypto"
	"github.com/mhiro2/envdesk/internal/testutil/cryptotest"
)

func newRootCommandWithCryptoAdapter(t *testing.T, adapter crypto.Adapter) *cobra.Command {
	t.Helper()
	return newRootCommand(func() crypto.Adapter {
		return adapter
	})
}

func newPlaintextRootCommand(t *testing.T) *cobra.Command {
	t.Helper()
	return newRootCommandWithCryptoAdapter(t, &cryptotest.PlaintextAdapter{})
}
