package security_test

import (
	"strings"
	"testing"

	"github.com/HybridMind1337/DataMask/internal/security"
)

func TestTargetProtectionGuard(t *testing.T) {
	guard := security.NewTargetProtectionGuard(nil)

	tests := []struct {
		target      string
		override    bool
		shouldError bool
	}{
		{"production", false, true},
		{"prod_db", false, true},
		{"live_backup", false, true},
		{"main_cluster", false, true},
		{"primary_db", false, true},
		{"staging_db", false, false},
		{"dev_local", false, false},
		{"test_db", false, false},
		{"production", true, false}, // Dangerous override allowed
	}

	for _, tt := range tests {
		err := guard.CheckTarget(tt.target, tt.override)
		if tt.shouldError && err == nil {
			t.Errorf("expected error for target %q, got nil", tt.target)
		}
		if !tt.shouldError && err != nil {
			t.Errorf("unexpected error for target %q: %v", tt.target, err)
		}
	}
}

func TestDeterministicKeyDeriver(t *testing.T) {
	t.Setenv("TEST_KEY", "super-secret-production-salt-key-999")

	deriver, err := security.NewKeyDeriver("TEST_KEY")
	if err != nil {
		t.Fatalf("failed to create deriver: %v", err)
	}

	val1 := deriver.DeriveHMAC("email", []byte("john@example.com"))
	val2 := deriver.DeriveHMAC("email", []byte("john@example.com"))
	val3 := deriver.DeriveHMAC("email", []byte("different@example.com"))

	if string(val1) != string(val2) {
		t.Fatalf("expected deterministic output for identical input")
	}
	if string(val1) == string(val3) {
		t.Fatalf("expected different output for different inputs")
	}

	int1 := deriver.DeriveInt64("phone", []byte("john@example.com"), 100, 200)
	int2 := deriver.DeriveInt64("phone", []byte("john@example.com"), 100, 200)
	if int1 != int2 {
		t.Fatalf("expected deterministic int output")
	}
	if int1 < 100 || int1 > 200 {
		t.Fatalf("derived int %d outside expected bounds [100, 200]", int1)
	}
}

func TestScrubMessage(t *testing.T) {
	raw := "Could not parse email user.alice@example.com or token eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.t-ID7"
	scrubbed := security.ScrubMessage(raw)

	if strings.Contains(scrubbed, "alice@example.com") {
		t.Errorf("PII email leaked in scrubbed message: %s", scrubbed)
	}
	if strings.Contains(scrubbed, "eyJhbGciOi") {
		t.Errorf("JWT token leaked in scrubbed message: %s", scrubbed)
	}
}
