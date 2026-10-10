package transformers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/HybridMind1337/DataMask/internal/masking"
)

// HashTransformer calculates SHA-256 hex digest.
type HashTransformer struct{}

func (t *HashTransformer) Name() string { return "hash" }
func (t *HashTransformer) Description() string {
	return "Replaces value with its SHA-256 hexadecimal representation"
}
func (t *HashTransformer) Transform(ctx context.Context, value any, meta masking.FieldMetadata, tCtx masking.TransformContext) (any, error) {
	if value == nil {
		return nil, nil
	}
	raw := fmt.Sprintf("%v", value)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:]), nil
}

// DeterministicHashTransformer calculates a keyed HMAC-SHA256 representation.
type DeterministicHashTransformer struct{}

func (t *DeterministicHashTransformer) Name() string { return "deterministic_hash" }
func (t *DeterministicHashTransformer) Description() string {
	return "Replaces value with a keyed HMAC hash preserving cross-table equality"
}
func (t *DeterministicHashTransformer) Transform(ctx context.Context, value any, meta masking.FieldMetadata, tCtx masking.TransformContext) (any, error) {
	if value == nil {
		return nil, nil
	}
	raw := []byte(fmt.Sprintf("%v", value))
	deriver := tCtx.KeyDeriver()
	if deriver == nil {
		// Fallback to SHA256 if no key deriver is configured
		sum := sha256.Sum256(raw)
		return hex.EncodeToString(sum[:]), nil
	}
	hash := deriver.DeriveHMAC("deterministic_hash", raw)
	return hex.EncodeToString(hash), nil
}
