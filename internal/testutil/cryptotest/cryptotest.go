package cryptotest

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mhiro2/envdesk/internal/crypto"
)

// PlaintextAdapter reads files as-is, simulating a no-op decrypt/encrypt.
// Use this when tests operate on plaintext env files directly.
type PlaintextAdapter struct{}

var _ crypto.Adapter = (*PlaintextAdapter)(nil)

func (*PlaintextAdapter) Check(context.Context) error { return nil }

func (*PlaintextAdapter) Decrypt(_ context.Context, path string) ([]byte, error) {
	// #nosec G304 -- test helpers only read fixture files created by the test.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %q: %w", path, err)
	}

	return data, nil
}

func (*PlaintextAdapter) Encrypt(_ context.Context, _ string, plaintext []byte) ([]byte, error) {
	return plaintext, nil
}

func (*PlaintextAdapter) Rekey(context.Context, string) error { return nil }

// StubAdapter lets tests define only the crypto operations they care about.
// Any unset operation fails fast so tests notice unexpected adapter usage.
type StubAdapter struct {
	CheckFunc   func(context.Context) error
	DecryptFunc func(context.Context, string) ([]byte, error)
	EncryptFunc func(context.Context, string, []byte) ([]byte, error)
	RekeyFunc   func(context.Context, string) error
}

var _ crypto.Adapter = (*StubAdapter)(nil)

func (s *StubAdapter) Check(ctx context.Context) error {
	if s == nil || s.CheckFunc == nil {
		return nil
	}

	return s.CheckFunc(ctx)
}

func (s *StubAdapter) Decrypt(ctx context.Context, path string) ([]byte, error) {
	if s == nil || s.DecryptFunc == nil {
		return nil, errors.New("unexpected decrypt call")
	}

	return s.DecryptFunc(ctx, path)
}

func (s *StubAdapter) Encrypt(ctx context.Context, path string, plaintext []byte) ([]byte, error) {
	if s == nil || s.EncryptFunc == nil {
		return nil, errors.New("unexpected encrypt call")
	}

	return s.EncryptFunc(ctx, path, plaintext)
}

func (s *StubAdapter) Rekey(ctx context.Context, path string) error {
	if s == nil || s.RekeyFunc == nil {
		return errors.New("unexpected rekey call")
	}

	return s.RekeyFunc(ctx, path)
}

// FakeEncryptAdapter simulates a real encrypt/decrypt cycle using base64 encoding.
// Decrypt decodes base64, Encrypt encodes to base64.
// This lets tests exercise the full pipeline with "encrypted" files on disk.
type FakeEncryptAdapter struct{}

var _ crypto.Adapter = (*FakeEncryptAdapter)(nil)

func (*FakeEncryptAdapter) Check(context.Context) error { return nil }

func (*FakeEncryptAdapter) Decrypt(_ context.Context, path string) ([]byte, error) {
	// #nosec G304 -- test helpers only read fixture files created by the test.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %q: %w", path, err)
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, fmt.Errorf("decrypt file %q: %w", path, err)
	}

	return decoded, nil
}

func (*FakeEncryptAdapter) Encrypt(_ context.Context, _ string, plaintext []byte) ([]byte, error) {
	encoded := base64.StdEncoding.EncodeToString(plaintext)
	return []byte(encoded), nil
}

func (*FakeEncryptAdapter) Rekey(context.Context, string) error { return nil }

// FakeEncryptContent encodes plaintext content using the same scheme as FakeEncryptAdapter.
// Use this to create "encrypted" fixture files for tests.
func FakeEncryptContent(plaintext string) string {
	return base64.StdEncoding.EncodeToString([]byte(plaintext))
}
