package crypto

import "context"

type Adapter interface {
	Check(ctx context.Context) error
	Decrypt(ctx context.Context, path string) ([]byte, error)
	Encrypt(ctx context.Context, path string, plaintext []byte) ([]byte, error)
	Rekey(ctx context.Context, path string) error
}
