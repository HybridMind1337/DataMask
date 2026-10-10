package transformers

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/HybridMind1337/DataMask/internal/masking"
)

// FakeIPv4Transformer generates non-routable documentation IPs (RFC 5737: 192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24).
type FakeIPv4Transformer struct{}

func (t *FakeIPv4Transformer) Name() string { return "fake_ipv4" }
func (t *FakeIPv4Transformer) Description() string {
	return "Replaces IP with an RFC 5737 non-routable documentation address (192.0.2.x)"
}
func (t *FakeIPv4Transformer) Transform(ctx context.Context, value any, meta masking.FieldMetadata, tCtx masking.TransformContext) (any, error) {
	if value == nil {
		return nil, nil
	}
	raw := []byte(fmt.Sprintf("%v", value))

	if tCtx.IsDeterministic() && tCtx.KeyDeriver() != nil {
		host := tCtx.KeyDeriver().DeriveInt64("ipv4_doc", raw, 2, 254)
		return fmt.Sprintf("192.0.2.%d", host), nil
	}

	h, _ := rand.Int(rand.Reader, big.NewInt(252))
	return fmt.Sprintf("192.0.2.%d", h.Int64()+2), nil
}

// FakeIPv6Transformer generates documentation IPv6 addresses (RFC 3849: 2001:db8::/32).
type FakeIPv6Transformer struct{}

func (t *FakeIPv6Transformer) Name() string { return "fake_ipv6" }
func (t *FakeIPv6Transformer) Description() string {
	return "Replaces IPv6 with an RFC 3849 documentation address (2001:db8::xxxx)"
}
func (t *FakeIPv6Transformer) Transform(ctx context.Context, value any, meta masking.FieldMetadata, tCtx masking.TransformContext) (any, error) {
	if value == nil {
		return nil, nil
	}
	raw := []byte(fmt.Sprintf("%v", value))

	if tCtx.IsDeterministic() && tCtx.KeyDeriver() != nil {
		val := tCtx.KeyDeriver().DeriveInt64("ipv6_doc", raw, 1, 65535)
		return fmt.Sprintf("2001:db8::%04x", val), nil
	}

	h, _ := rand.Int(rand.Reader, big.NewInt(65534))
	return fmt.Sprintf("2001:db8::%04x", h.Int64()+1), nil
}
