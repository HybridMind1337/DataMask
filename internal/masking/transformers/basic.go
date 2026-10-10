package transformers

import (
	"context"
	"fmt"

	"github.com/HybridMind1337/DataMask/internal/masking"
)

// PreserveTransformer retains the original value without alteration.
type PreserveTransformer struct{}

func (t *PreserveTransformer) Name() string { return "preserve" }
func (t *PreserveTransformer) Description() string {
	return "Preserves the original column value unmodified"
}
func (t *PreserveTransformer) Transform(ctx context.Context, value any, meta masking.FieldMetadata, tCtx masking.TransformContext) (any, error) {
	return value, nil
}

// NullTransformer replaces any value with nil (SQL NULL).
type NullTransformer struct{}

func (t *NullTransformer) Name() string { return "null" }
func (t *NullTransformer) Description() string {
	return "Replaces the value with NULL (fails if column is non-nullable)"
}
func (t *NullTransformer) Transform(ctx context.Context, value any, meta masking.FieldMetadata, tCtx masking.TransformContext) (any, error) {
	if !meta.IsNullable {
		return nil, fmt.Errorf("cannot apply null strategy to non-nullable column")
	}
	return nil, nil
}

// RedactTransformer replaces values with fixed redaction markers.
type RedactTransformer struct{}

func (t *RedactTransformer) Name() string { return "redact" }
func (t *RedactTransformer) Description() string {
	return "Replaces the value with [REDACTED]"
}
func (t *RedactTransformer) Transform(ctx context.Context, value any, meta masking.FieldMetadata, tCtx masking.TransformContext) (any, error) {
	if value == nil {
		return nil, nil
	}
	return "[REDACTED]", nil
}

// ConstantTransformer replaces values with a configured constant.
type ConstantTransformer struct {
	ConstantValue any
}

func (t *ConstantTransformer) Name() string { return "constant" }
func (t *ConstantTransformer) Description() string {
	return "Replaces value with a fixed static constant"
}
func (t *ConstantTransformer) Transform(ctx context.Context, value any, meta masking.FieldMetadata, tCtx masking.TransformContext) (any, error) {
	if t.ConstantValue == nil {
		return "CONSTANT", nil
	}
	return t.ConstantValue, nil
}
