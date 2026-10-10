package transformers

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/HybridMind1337/DataMask/internal/masking"
)

// RandomUUIDTransformer generates synthetic RFC 4122 v4 UUIDs.
type RandomUUIDTransformer struct{}

func (t *RandomUUIDTransformer) Name() string { return "random_uuid" }
func (t *RandomUUIDTransformer) Description() string {
	return "Generates a synthetic UUID v4"
}
func (t *RandomUUIDTransformer) Transform(ctx context.Context, value any, meta masking.FieldMetadata, tCtx masking.TransformContext) (any, error) {
	if value == nil {
		return nil, nil
	}
	var u [16]byte
	if tCtx.IsDeterministic() && tCtx.KeyDeriver() != nil {
		raw := []byte(fmt.Sprintf("%v", value))
		hash := tCtx.KeyDeriver().DeriveHMAC("uuid_v4", raw)
		copy(u[:], hash[:16])
	} else {
		_, _ = rand.Read(u[:])
	}
	u[6] = (u[6] & 0x0f) | 0x40 // Version 4
	u[8] = (u[8] & 0x3f) | 0x80 // Variant RFC 4122
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:16]), nil
}

// RandomDateTransformer shifts or generates plausible dates within realistic windows.
type RandomDateTransformer struct{}

func (t *RandomDateTransformer) Name() string { return "random_date" }
func (t *RandomDateTransformer) Description() string {
	return "Generates plausible synthetic dates preserving historical distribution"
}
func (t *RandomDateTransformer) Transform(ctx context.Context, value any, meta masking.FieldMetadata, tCtx masking.TransformContext) (any, error) {
	if value == nil {
		return nil, nil
	}
	// Plausible window: between 1980 and 2024
	base := time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
	daysRange := int64(44 * 365)

	var days int64
	if tCtx.IsDeterministic() && tCtx.KeyDeriver() != nil {
		raw := []byte(fmt.Sprintf("%v", value))
		days = tCtx.KeyDeriver().DeriveInt64("date_shift", raw, 0, daysRange)
	} else {
		n, _ := rand.Int(rand.Reader, big.NewInt(daysRange))
		days = n.Int64()
	}

	res := base.AddDate(0, 0, int(days))
	if _, ok := value.(time.Time); ok {
		return res, nil
	}
	return res.Format("2006-01-02"), nil
}

// RandomIntegerTransformer generates realistic integer within bounds.
type RandomIntegerTransformer struct {
	Min int64
	Max int64
}

func (t *RandomIntegerTransformer) Name() string { return "random_integer" }
func (t *RandomIntegerTransformer) Description() string {
	return "Generates bounded random integers"
}
func (t *RandomIntegerTransformer) Transform(ctx context.Context, value any, meta masking.FieldMetadata, tCtx masking.TransformContext) (any, error) {
	if value == nil {
		return nil, nil
	}
	minVal, maxVal := t.Min, t.Max
	if minVal >= maxVal {
		minVal = 1
		maxVal = 1000
	}
	if tCtx.IsDeterministic() && tCtx.KeyDeriver() != nil {
		raw := []byte(fmt.Sprintf("%v", value))
		return tCtx.KeyDeriver().DeriveInt64("int_gen", raw, minVal, maxVal), nil
	}
	diff := maxVal - minVal + 1
	n, _ := rand.Int(rand.Reader, big.NewInt(diff))
	return minVal + n.Int64(), nil
}

// DevAuthTransformer replaces authentication hashes with a known development password hash.
type DevAuthTransformer struct{}

func (t *DevAuthTransformer) Name() string { return "dev_auth" }
func (t *DevAuthTransformer) Description() string {
	return "Replaces password hashes with a known development staging hash"
}
func (t *DevAuthTransformer) Transform(ctx context.Context, value any, meta masking.FieldMetadata, tCtx masking.TransformContext) (any, error) {
	if value == nil {
		return nil, nil
	}
	hash := tCtx.DevPasswordHash()
	if hash == "" {
		// Default bcrypt hash for "secret" if not explicitly configured
		return "$2y$10$TKh8H1.PfQx37YgCzwiKb.KjNyWgaHb9cbcoQgdIVFlYg7B77UdFm", nil
	}
	return hash, nil
}
