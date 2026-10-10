package masking_test

import (
	"context"
	"testing"

	"github.com/HybridMind1337/DataMask/internal/masking"
	"github.com/HybridMind1337/DataMask/internal/masking/transformers"
	"github.com/HybridMind1337/DataMask/internal/security"
)

func BenchmarkDeterministicEmailTransformer(b *testing.B) {
	b.Setenv("BENCH_KEY", "high-entropy-benchmark-key-999")
	deriver, _ := security.NewKeyDeriver("BENCH_KEY")
	tCtx := masking.NewTransformContext(true, deriver, "")
	ctx := context.Background()

	t := &transformers.FakeEmailTransformer{}
	meta := masking.FieldMetadata{
		TableName:  "users",
		ColumnName: "email",
		IsUnique:   false,
	}

	raw := "benchmark.user@production-company.com"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = t.Transform(ctx, raw, meta, tCtx)
	}
}

func BenchmarkDeterministicHashTransformer(b *testing.B) {
	b.Setenv("BENCH_KEY", "high-entropy-benchmark-key-999")
	deriver, _ := security.NewKeyDeriver("BENCH_KEY")
	tCtx := masking.NewTransformContext(true, deriver, "")
	ctx := context.Background()

	t := &transformers.DeterministicHashTransformer{}
	meta := masking.FieldMetadata{
		TableName:  "users",
		ColumnName: "identifier",
	}

	raw := "104918294819"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = t.Transform(ctx, raw, meta, tCtx)
	}
}
