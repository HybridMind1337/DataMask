package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash"
	"os"
	"strings"
	"sync"
)

// TargetProtectionGuard prevents writing to databases that resemble production.
type TargetProtectionGuard struct {
	ProtectedKeywords []string
}

// NewTargetProtectionGuard creates a guard with specified or default forbidden keywords.
func NewTargetProtectionGuard(keywords []string) *TargetProtectionGuard {
	if len(keywords) == 0 {
		keywords = []string{"prod", "production", "live", "main", "primary", "master"}
	}
	return &TargetProtectionGuard{ProtectedKeywords: keywords}
}

// CheckTarget returns an error if targetName matches any protected keyword unless explicitly overridden.
func (g *TargetProtectionGuard) CheckTarget(targetName string, allowDangerousOverride bool) error {
	low := strings.ToLower(targetName)
	for _, kw := range g.ProtectedKeywords {
		if strings.Contains(low, strings.ToLower(kw)) {
			if !allowDangerousOverride {
				return fmt.Errorf("CRITICAL SECURITY ERROR: Target database %q appears to be a protected environment (matches keyword %q). DataMask will NOT write to this target. Use --force-dangerous-target-override if you are absolutely certain", targetName, kw)
			}
		}
	}
	return nil
}

// KeyDeriver provides high-throughput HMAC-based deterministic entropy derivation.
type KeyDeriver struct {
	secretKey []byte
	hmacPool  sync.Pool
}

// NewKeyDeriver loads the key from the specified environment variable or uses fallback entropy.
func NewKeyDeriver(envVar string) (*KeyDeriver, error) {
	if envVar == "" {
		envVar = "DATAMASK_TRANSFORM_KEY"
	}
	key := os.Getenv(envVar)
	if key == "" {
		return nil, fmt.Errorf("deterministic masking requested but environment variable %s is not set", envVar)
	}
	keyBytes := []byte(key)
	kd := &KeyDeriver{
		secretKey: keyBytes,
	}
	kd.hmacPool.New = func() any {
		return hmac.New(sha256.New, keyBytes)
	}
	return kd, nil
}

// DeriveHMAC derives a stable 32-byte hash using pooled HMAC hashers with zero per-call key allocation.
func (k *KeyDeriver) DeriveHMAC(contextDomain string, input []byte) []byte {
	h := k.hmacPool.Get().(hash.Hash)
	defer func() {
		h.Reset()
		k.hmacPool.Put(h)
	}()

	h.Write([]byte(contextDomain))
	h.Write([]byte(":"))
	h.Write(input)
	return h.Sum(nil)
}

// DeriveInt64 derives a deterministic int64 in range [min, max] with zero unnecessary allocations.
func (k *KeyDeriver) DeriveInt64(contextDomain string, input []byte, minVal, maxVal int64) int64 {
	if minVal >= maxVal {
		return minVal
	}
	hashBytes := k.DeriveHMAC(contextDomain, input)
	val := binary.BigEndian.Uint64(hashBytes[:8])
	rangeSize := uint64(maxVal - minVal + 1)
	return minVal + int64(val%rangeSize)
}
