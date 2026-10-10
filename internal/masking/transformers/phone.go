package transformers

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/HybridMind1337/DataMask/internal/masking"
)

// FakePhoneTransformer produces format-preserving phone numbers from non-routable fictional blocks.
// UK: +44 7700 900xxx (Ofcom dramatic numbers)
// US: +1 555 0100 to 0199 (fictional)
type FakePhoneTransformer struct{}

func (t *FakePhoneTransformer) Name() string { return "fake_phone" }
func (t *FakePhoneTransformer) Description() string {
	return "Replaces phone numbers with non-routable fictional numbers (+44 7700 900xxx or +1 555-01xx)"
}
func (t *FakePhoneTransformer) Transform(ctx context.Context, value any, meta masking.FieldMetadata, tCtx masking.TransformContext) (any, error) {
	if value == nil {
		return nil, nil
	}
	raw := []byte(fmt.Sprintf("%v", value))

	if tCtx.IsDeterministic() && tCtx.KeyDeriver() != nil {
		suffix := tCtx.KeyDeriver().DeriveInt64("phone_uk", raw, 100000, 999999)
		return fmt.Sprintf("+44 7700 %d", suffix), nil
	}

	n, _ := rand.Int(rand.Reader, big.NewInt(900000))
	return fmt.Sprintf("+44 7700 %d", n.Int64()+100000), nil
}
