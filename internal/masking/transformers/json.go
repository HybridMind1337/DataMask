package transformers

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/HybridMind1337/DataMask/internal/masking"
)

// JSONMaskTransformer parses JSON or JSONB structures and recursively masks sensitive keys.
type JSONMaskTransformer struct {
	EmailTransformer masking.Transformer
	PhoneTransformer masking.Transformer
	NameTransformer  masking.Transformer
}

func NewJSONMaskTransformer() *JSONMaskTransformer {
	return &JSONMaskTransformer{
		EmailTransformer: &FakeEmailTransformer{},
		PhoneTransformer: &FakePhoneTransformer{},
		NameTransformer:  &FakeNameTransformer{},
	}
}

func (t *JSONMaskTransformer) Name() string { return "json_mask" }
func (t *JSONMaskTransformer) Description() string {
	return "Recursively parses JSON and masks sensitive keys (emails, tokens, phones, names)"
}

func (t *JSONMaskTransformer) Transform(ctx context.Context, value any, meta masking.FieldMetadata, tCtx masking.TransformContext) (any, error) {
	if value == nil {
		return nil, nil
	}

	var rawBytes []byte
	switch v := value.(type) {
	case string:
		rawBytes = []byte(v)
	case []byte:
		rawBytes = v
	default:
		// Try serializing if it's already an arbitrary object/map
		b, err := json.Marshal(v)
		if err != nil {
			return value, nil
		}
		rawBytes = b
	}

	trimmed := strings.TrimSpace(string(rawBytes))
	if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") {
		// Not valid JSON string, return untouched
		return value, nil
	}

	var root any
	if err := json.Unmarshal(rawBytes, &root); err != nil {
		return value, nil
	}

	root = t.maskNode(ctx, root, meta, tCtx)

	resBytes, err := json.Marshal(root)
	if err != nil {
		return value, nil
	}

	if _, ok := value.([]byte); ok {
		return resBytes, nil
	}
	return string(resBytes), nil
}

func (t *JSONMaskTransformer) maskNode(ctx context.Context, node any, meta masking.FieldMetadata, tCtx masking.TransformContext) any {
	switch v := node.(type) {
	case map[string]any:
		for k, child := range v {
			lowKey := strings.ToLower(k)
			if t.isCredentialKey(lowKey) {
				v[k] = "[REDACTED]"
			} else if strings.Contains(lowKey, "email") {
				if s, ok := child.(string); ok && s != "" {
					masked, err := t.EmailTransformer.Transform(ctx, s, meta, tCtx)
					if err == nil {
						v[k] = masked
					}
				}
			} else if strings.Contains(lowKey, "phone") || strings.Contains(lowKey, "mobile") {
				if s, ok := child.(string); ok && s != "" {
					masked, err := t.PhoneTransformer.Transform(ctx, s, meta, tCtx)
					if err == nil {
						v[k] = masked
					}
				}
			} else if strings.Contains(lowKey, "name") && !strings.Contains(lowKey, "table") && !strings.Contains(lowKey, "file") {
				if s, ok := child.(string); ok && s != "" {
					masked, err := t.NameTransformer.Transform(ctx, s, meta, tCtx)
					if err == nil {
						v[k] = masked
					}
				}
			} else {
				v[k] = t.maskNode(ctx, child, meta, tCtx)
			}
		}
		return v
	case []any:
		for i, elem := range v {
			v[i] = t.maskNode(ctx, elem, meta, tCtx)
		}
		return v
	default:
		return node
	}
}

func (t *JSONMaskTransformer) isCredentialKey(key string) bool {
	return strings.Contains(key, "password") ||
		strings.Contains(key, "secret") ||
		strings.Contains(key, "token") ||
		strings.Contains(key, "api_key") ||
		strings.Contains(key, "auth") ||
		strings.Contains(key, "jwt")
}
