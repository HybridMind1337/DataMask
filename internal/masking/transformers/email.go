package transformers

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"github.com/HybridMind1337/DataMask/internal/masking"
)

var safeDomains = []string{
	"example.test", "example.com", "example.org", "sample.test", "invalidsite.test",
}

// FakeEmailTransformer generates privacy-safe emails ending in .test or example.com.
type FakeEmailTransformer struct{}

func (t *FakeEmailTransformer) Name() string { return "fake_email" }
func (t *FakeEmailTransformer) Description() string {
	return "Replaces email with a privacy-safe address using reserved domains (.test)"
}
func (t *FakeEmailTransformer) Transform(ctx context.Context, value any, meta masking.FieldMetadata, tCtx masking.TransformContext) (any, error) {
	if value == nil {
		return nil, nil
	}
	raw := []byte(fmt.Sprintf("%v", value))

	tracker := tCtx.CollisionTracker()
	if meta.IsUnique && tracker != nil {
		if existing, ok := tracker.GetExisting(meta.TableName, meta.ColumnName, string(raw)); ok {
			return existing, nil
		}
	}

	maxAttempts := 10

	for attempt := 0; attempt < maxAttempts; attempt++ {
		var candidate string
		if tCtx.IsDeterministic() && tCtx.KeyDeriver() != nil {
			// Deterministic derivation
			fIdx := tCtx.KeyDeriver().DeriveInt64("email_fn", raw, 0, int64(len(sampleFirstNames)-1))
			lIdx := tCtx.KeyDeriver().DeriveInt64("email_ln", raw, 0, int64(len(sampleLastNames)-1))
			domIdx := tCtx.KeyDeriver().DeriveInt64("email_dom", raw, 0, int64(len(safeDomains)-1))
			num := tCtx.KeyDeriver().DeriveInt64("email_num", raw, 10, 9999)

			if attempt == 0 {
				candidate = strings.ToLower(fmt.Sprintf("%s.%s@%s", sampleFirstNames[fIdx], sampleLastNames[lIdx], safeDomains[domIdx]))
			} else {
				candidate = strings.ToLower(fmt.Sprintf("%s.%s%d@%s", sampleFirstNames[fIdx], sampleLastNames[lIdx], num+int64(attempt), safeDomains[domIdx]))
			}
		} else {
			// Random generation
			fn, _ := rand.Int(rand.Reader, big.NewInt(int64(len(sampleFirstNames))))
			ln, _ := rand.Int(rand.Reader, big.NewInt(int64(len(sampleLastNames))))
			num, _ := rand.Int(rand.Reader, big.NewInt(9000))
			candidate = strings.ToLower(fmt.Sprintf("%s.%s%d@%s", sampleFirstNames[fn.Int64()], sampleLastNames[ln.Int64()], num.Int64()+100, safeDomains[0]))
		}

		if meta.IsUnique && tracker != nil {
			if tracker.IsDuplicate(meta.TableName, meta.ColumnName, candidate) {
				continue
			}
			tracker.Record(meta.TableName, meta.ColumnName, string(raw), candidate)
		}
		return candidate, nil
	}

	return nil, fmt.Errorf("exhausted unique collision retry budget for email generation")
}
