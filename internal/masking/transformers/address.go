package transformers

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/HybridMind1337/DataMask/internal/masking"
)

var sampleStreets = []string{
	"High Street", "Station Road", "Main Street", "Park Road", "Church Street",
	"London Road", "Victoria Road", "Green Lane", "Manor Road", "Church Road",
	"Broadway", "Oak Avenue", "Maple Street", "Cedar Lane", "Sunset Boulevard",
}

var sampleCities = []string{
	"Springfield", "Riverdale", "Fairview", "Centerville", "Franklin",
	"Greenville", "Bristol", "Chester", "York", "Winchester",
}

// FakeAddressTransformer generates synthetic street addresses.
type FakeAddressTransformer struct{}

func (t *FakeAddressTransformer) Name() string { return "fake_address" }
func (t *FakeAddressTransformer) Description() string {
	return "Generates realistic synthetic street addresses"
}
func (t *FakeAddressTransformer) Transform(ctx context.Context, value any, meta masking.FieldMetadata, tCtx masking.TransformContext) (any, error) {
	if value == nil {
		return nil, nil
	}
	raw := []byte(fmt.Sprintf("%v", value))

	if tCtx.IsDeterministic() && tCtx.KeyDeriver() != nil {
		num := tCtx.KeyDeriver().DeriveInt64("addr_num", raw, 1, 999)
		stIdx := tCtx.KeyDeriver().DeriveInt64("addr_st", raw, 0, int64(len(sampleStreets)-1))
		cIdx := tCtx.KeyDeriver().DeriveInt64("addr_city", raw, 0, int64(len(sampleCities)-1))
		return fmt.Sprintf("%d %s, %s", num, sampleStreets[stIdx], sampleCities[cIdx]), nil
	}

	num, _ := rand.Int(rand.Reader, big.NewInt(998))
	st, _ := rand.Int(rand.Reader, big.NewInt(int64(len(sampleStreets))))
	c, _ := rand.Int(rand.Reader, big.NewInt(int64(len(sampleCities))))
	return fmt.Sprintf("%d %s, %s", num.Int64()+1, sampleStreets[st.Int64()], sampleCities[c.Int64()]), nil
}

// FakePostcodeTransformer generates non-routable postal codes.
type FakePostcodeTransformer struct{}

func (t *FakePostcodeTransformer) Name() string { return "fake_postcode" }
func (t *FakePostcodeTransformer) Description() string {
	return "Generates format-preserving postal codes"
}
func (t *FakePostcodeTransformer) Transform(ctx context.Context, value any, meta masking.FieldMetadata, tCtx masking.TransformContext) (any, error) {
	if value == nil {
		return nil, nil
	}
	raw := []byte(fmt.Sprintf("%v", value))

	if tCtx.IsDeterministic() && tCtx.KeyDeriver() != nil {
		area := tCtx.KeyDeriver().DeriveInt64("post_area", raw, 10, 99)
		unit := tCtx.KeyDeriver().DeriveInt64("post_unit", raw, 1, 9)
		return fmt.Sprintf("TE%d %dST", area, unit), nil
	}

	a, _ := rand.Int(rand.Reader, big.NewInt(89))
	u, _ := rand.Int(rand.Reader, big.NewInt(9))
	return fmt.Sprintf("TE%d %dST", a.Int64()+10, u.Int64()+1), nil
}
