package transformers

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/HybridMind1337/DataMask/internal/masking"
)

// NumericNoiseTransformer adds a percentage variance (e.g. +/- 10%) while preserving signs and plausible ranges.
type NumericNoiseTransformer struct {
	Percentage float64 // e.g. 0.10 for +/- 10%
}

func (t *NumericNoiseTransformer) Name() string { return "perturb_numeric" }
func (t *NumericNoiseTransformer) Description() string {
	return "Adds bounded Gaussian/uniform noise (+/- 10%) to numeric values preserving realistic distributions"
}

func (t *NumericNoiseTransformer) Transform(ctx context.Context, value any, meta masking.FieldMetadata, tCtx masking.TransformContext) (any, error) {
	if value == nil {
		return nil, nil
	}

	pct := t.Percentage
	if pct <= 0 {
		pct = 0.10 // default 10% variance
	}

	var numVal float64
	var isInt bool

	switch v := value.(type) {
	case int:
		numVal = float64(v)
		isInt = true
	case int32:
		numVal = float64(v)
		isInt = true
	case int64:
		numVal = float64(v)
		isInt = true
	case float32:
		numVal = float64(v)
	case float64:
		numVal = v
	default:
		// Attempt string parse
		s := fmt.Sprintf("%v", v)
		var f float64
		if _, err := fmt.Sscanf(s, "%f", &f); err == nil {
			numVal = f
		} else {
			return value, nil
		}
	}

	// Deterministic vs random factor in [-pct, +pct]
	var factor float64
	if tCtx.IsDeterministic() && tCtx.KeyDeriver() != nil {
		raw := []byte(fmt.Sprintf("%v", value))
		// Map int to [-1000, 1000]
		dev := tCtx.KeyDeriver().DeriveInt64("noise_pct", raw, -1000, 1000)
		factor = (float64(dev) / 1000.0) * pct
	} else {
		n, _ := rand.Int(rand.Reader, big.NewInt(2000))
		factor = ((float64(n.Int64()) - 1000.0) / 1000.0) * pct
	}

	res := numVal * (1.0 + factor)

	if isInt {
		return int64(math.Round(res)), nil
	}
	return math.Round(res*100) / 100, nil // 2 decimal precision
}

// DateShiftTransformer shifts dates by a deterministic or random offset window (+/- 30 days).
type DateShiftTransformer struct {
	MaxDaysShift int64
}

func (t *DateShiftTransformer) Name() string { return "date_shift" }
func (t *DateShiftTransformer) Description() string {
	return "Shifts dates by a bounded day offset preserving sequence intervals and day of week"
}

func (t *DateShiftTransformer) Transform(ctx context.Context, value any, meta masking.FieldMetadata, tCtx masking.TransformContext) (any, error) {
	if value == nil {
		return nil, nil
	}

	maxDays := t.MaxDaysShift
	if maxDays <= 0 {
		maxDays = 30
	}

	var dt time.Time
	var isTimeType bool

	switch v := value.(type) {
	case time.Time:
		dt = v
		isTimeType = true
	case string:
		// Try parsing common SQL date formats
		formats := []string{
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05Z",
			"2006-01-02",
		}
		var parsed bool
		for _, f := range formats {
			if tParsed, err := time.Parse(f, v); err == nil {
				dt = tParsed
				parsed = true
				break
			}
		}
		if !parsed {
			return value, nil
		}
	default:
		return value, nil
	}

	var daysOffset int64
	if tCtx.IsDeterministic() && tCtx.KeyDeriver() != nil {
		raw := []byte(fmt.Sprintf("%v", value))
		daysOffset = tCtx.KeyDeriver().DeriveInt64("date_shift_days", raw, -maxDays, maxDays)
	} else {
		diff := maxDays*2 + 1
		n, _ := rand.Int(rand.Reader, big.NewInt(diff))
		daysOffset = n.Int64() - maxDays
	}

	shifted := dt.AddDate(0, 0, int(daysOffset))

	if isTimeType {
		return shifted, nil
	}
	return shifted.Format("2006-01-02 15:04:05"), nil
}
