package transformers

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"github.com/HybridMind1337/DataMask/internal/masking"
)

var sampleFirstNames = []string{
	"James", "Mary", "Robert", "Patricia", "John", "Jennifer", "Michael", "Linda",
	"David", "Elizabeth", "William", "Barbara", "Richard", "Susan", "Joseph", "Jessica",
	"Thomas", "Sarah", "Charles", "Karen", "Christopher", "Nancy", "Daniel", "Lisa",
	"Matthew", "Betty", "Anthony", "Margaret", "Mark", "Sandra", "Donald", "Ashley",
	"Steven", "Kimberly", "Paul", "Emily", "Andrew", "Donna", "Joshua", "Michelle",
	"Oliver", "Emma", "Jack", "Charlotte", "Harry", "Amelia", "Jacob", "Sophia",
	"Alexander", "Olivia",
}

var sampleLastNames = []string{
	"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis",
	"Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez", "Wilson", "Anderson",
	"Thomas", "Taylor", "Moore", "Jackson", "Martin", "Lee", "Perez", "Thompson",
	"White", "Harris", "Sanchez", "Clark", "Ramirez", "Lewis", "Robinson", "Walker",
	"Young", "Allen", "King", "Wright", "Scott", "Torres", "Nguyen", "Hill", "Flores",
	"Green", "Adams", "Nelson", "Baker", "Hall", "Rivera", "Campbell", "Mitchell",
	"Carter", "Roberts",
}

// FakeFirstNameTransformer generates a plausible realistic first name.
type FakeFirstNameTransformer struct{}

func (t *FakeFirstNameTransformer) Name() string { return "fake_first_name" }
func (t *FakeFirstNameTransformer) Description() string {
	return "Generates a realistic synthetic first name"
}
func (t *FakeFirstNameTransformer) Transform(ctx context.Context, value any, meta masking.FieldMetadata, tCtx masking.TransformContext) (any, error) {
	if value == nil {
		return nil, nil
	}
	raw := []byte(fmt.Sprintf("%v", value))
	if tCtx.IsDeterministic() && tCtx.KeyDeriver() != nil {
		idx := tCtx.KeyDeriver().DeriveInt64("first_name", raw, 0, int64(len(sampleFirstNames)-1))
		return sampleFirstNames[idx], nil
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(sampleFirstNames))))
	return sampleFirstNames[n.Int64()], nil
}

// FakeLastNameTransformer generates a plausible realistic last name.
type FakeLastNameTransformer struct{}

func (t *FakeLastNameTransformer) Name() string { return "fake_last_name" }
func (t *FakeLastNameTransformer) Description() string {
	return "Generates a realistic synthetic last name"
}
func (t *FakeLastNameTransformer) Transform(ctx context.Context, value any, meta masking.FieldMetadata, tCtx masking.TransformContext) (any, error) {
	if value == nil {
		return nil, nil
	}
	raw := []byte(fmt.Sprintf("%v", value))
	if tCtx.IsDeterministic() && tCtx.KeyDeriver() != nil {
		idx := tCtx.KeyDeriver().DeriveInt64("last_name", raw, 0, int64(len(sampleLastNames)-1))
		return sampleLastNames[idx], nil
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(sampleLastNames))))
	return sampleLastNames[n.Int64()], nil
}

// FakeNameTransformer generates a realistic full name (First Last).
type FakeNameTransformer struct{}

func (t *FakeNameTransformer) Name() string { return "fake_name" }
func (t *FakeNameTransformer) Description() string {
	return "Generates a realistic synthetic full name"
}
func (t *FakeNameTransformer) Transform(ctx context.Context, value any, meta masking.FieldMetadata, tCtx masking.TransformContext) (any, error) {
	if value == nil {
		return nil, nil
	}
	raw := []byte(fmt.Sprintf("%v", value))
	if tCtx.IsDeterministic() && tCtx.KeyDeriver() != nil {
		fIdx := tCtx.KeyDeriver().DeriveInt64("first_name", raw, 0, int64(len(sampleFirstNames)-1))
		lIdx := tCtx.KeyDeriver().DeriveInt64("last_name", raw, 0, int64(len(sampleLastNames)-1))
		return fmt.Sprintf("%s %s", sampleFirstNames[fIdx], sampleLastNames[lIdx]), nil
	}
	fn, _ := rand.Int(rand.Reader, big.NewInt(int64(len(sampleFirstNames))))
	ln, _ := rand.Int(rand.Reader, big.NewInt(int64(len(sampleLastNames))))
	return fmt.Sprintf("%s %s", sampleFirstNames[fn.Int64()], sampleLastNames[ln.Int64()]), nil
}

// FakeUsernameTransformer generates a synthetic username.
type FakeUsernameTransformer struct{}

func (t *FakeUsernameTransformer) Name() string { return "fake_username" }
func (t *FakeUsernameTransformer) Description() string {
	return "Generates a realistic synthetic username"
}
func (t *FakeUsernameTransformer) Transform(ctx context.Context, value any, meta masking.FieldMetadata, tCtx masking.TransformContext) (any, error) {
	if value == nil {
		return nil, nil
	}
	raw := []byte(fmt.Sprintf("%v", value))
	if tCtx.IsDeterministic() && tCtx.KeyDeriver() != nil {
		fIdx := tCtx.KeyDeriver().DeriveInt64("first_name", raw, 0, int64(len(sampleFirstNames)-1))
		num := tCtx.KeyDeriver().DeriveInt64("user_suffix", raw, 10, 9999)
		return strings.ToLower(fmt.Sprintf("%s%d", sampleFirstNames[fIdx], num)), nil
	}
	fn, _ := rand.Int(rand.Reader, big.NewInt(int64(len(sampleFirstNames))))
	num, _ := rand.Int(rand.Reader, big.NewInt(9000))
	return strings.ToLower(fmt.Sprintf("%s%d", sampleFirstNames[fn.Int64()], num.Int64()+1000)), nil
}
