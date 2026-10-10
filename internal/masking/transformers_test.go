package masking_test

import (
	"context"
	"strings"
	"testing"

	"github.com/HybridMind1337/DataMask/internal/masking"
	"github.com/HybridMind1337/DataMask/internal/masking/transformers"
	"github.com/HybridMind1337/DataMask/internal/security"
)

func TestTransformers(t *testing.T) {
	reg := masking.NewRegistry()
	if err := transformers.RegisterBuiltins(reg); err != nil {
		t.Fatalf("failed to register builtins: %v", err)
	}

	t.Setenv("TEST_KEY", "deterministic-test-key-12345")
	deriver, _ := security.NewKeyDeriver("TEST_KEY")
	tCtx := masking.NewTransformContext(true, deriver, "")
	ctx := context.Background()

	// 1. Email transformer
	emailT, _ := reg.Get("fake_email")
	meta := masking.FieldMetadata{
		TableName:  "users",
		ColumnName: "email",
		IsUnique:   true,
	}

	res1, err := emailT.Transform(ctx, "alice@realproduction.com", meta, tCtx)
	if err != nil {
		t.Fatalf("email transform error: %v", err)
	}
	emailStr, ok := res1.(string)
	if !ok || !strings.Contains(emailStr, "@") {
		t.Fatalf("invalid generated email: %v", res1)
	}
	if strings.Contains(emailStr, "realproduction.com") {
		t.Fatalf("original domain leaked: %s", emailStr)
	}

	// Deterministic equality check
	res2, _ := emailT.Transform(ctx, "alice@realproduction.com", meta, tCtx)
	if res1 != res2 {
		t.Fatalf("deterministic email produced different values: %v vs %v", res1, res2)
	}

	// 2. Phone transformer
	phoneT, _ := reg.Get("fake_phone")
	pRes, err := phoneT.Transform(ctx, "+44 7911 123456", meta, tCtx)
	if err != nil {
		t.Fatalf("phone transform error: %v", err)
	}
	pStr := pRes.(string)
	if !strings.HasPrefix(pStr, "+44 7700") {
		t.Fatalf("expected UK dramatic non-routable range (+44 7700), got %s", pStr)
	}

	// 3. IPv4 transformer
	ipT, _ := reg.Get("fake_ipv4")
	ipRes, _ := ipT.Transform(ctx, "81.2.3.4", meta, tCtx)
	if !strings.HasPrefix(ipRes.(string), "192.0.2.") {
		t.Fatalf("expected RFC 5737 documentation IP, got %v", ipRes)
	}

	// 4. Redact transformer
	redactT, _ := reg.Get("redact")
	rRes, _ := redactT.Transform(ctx, "super-secret-token", meta, tCtx)
	if rRes != "[REDACTED]" {
		t.Fatalf("expected [REDACTED], got %v", rRes)
	}

	// 5. Pattern scrub transformer
	scrubT, _ := reg.Get("pattern_scrub")
	freeText := "Please contact me at customer@service.org or call +44 7911 123456 immediately."
	sRes, _ := scrubT.Transform(ctx, freeText, meta, tCtx)
	sStr := sRes.(string)
	if strings.Contains(sStr, "customer@service.org") {
		t.Fatalf("email leaked in scrubbed free-text: %s", sStr)
	}
	if !strings.Contains(sStr, "[REDACTED_EMAIL]") {
		t.Fatalf("expected [REDACTED_EMAIL] placeholder in scrubbed text")
	}

	// 6. JSON deep masking transformer
	jsonT, _ := reg.Get("json_mask")
	rawJSON := `{"user": {"name": "Alice Smith", "email": "alice@production.org", "api_token": "secret123"}}`
	jRes, err := jsonT.Transform(ctx, rawJSON, meta, tCtx)
	if err != nil {
		t.Fatalf("json transform error: %v", err)
	}
	jStr := jRes.(string)
	if strings.Contains(jStr, "alice@production.org") {
		t.Fatalf("original email leaked in JSON output: %s", jStr)
	}
	if strings.Contains(jStr, "secret123") {
		t.Fatalf("api token leaked in JSON output: %s", jStr)
	}
	if !strings.Contains(jStr, "[REDACTED]") {
		t.Fatalf("expected [REDACTED] in masked JSON token: %s", jStr)
	}
}
