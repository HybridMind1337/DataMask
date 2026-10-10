package transformers

import "github.com/HybridMind1337/DataMask/internal/masking"

// RegisterBuiltins populates the registry with all standard transformers.
func RegisterBuiltins(r *masking.Registry) error {
	builtins := []masking.Transformer{
		&PreserveTransformer{},
		&NullTransformer{},
		&RedactTransformer{},
		&ConstantTransformer{},
		&HashTransformer{},
		&DeterministicHashTransformer{},
		&FakeFirstNameTransformer{},
		&FakeLastNameTransformer{},
		&FakeNameTransformer{},
		&FakeUsernameTransformer{},
		&FakeEmailTransformer{},
		&FakePhoneTransformer{},
		&FakeAddressTransformer{},
		&FakePostcodeTransformer{},
		&FakeIPv4Transformer{},
		&FakeIPv6Transformer{},
		&RandomUUIDTransformer{},
		&RandomDateTransformer{},
		&RandomIntegerTransformer{Min: 1, Max: 1000},
		&DevAuthTransformer{},
		&FreeTextScrubTransformer{},
		&SyntheticTextTransformer{},
		NewJSONMaskTransformer(),
		&NumericNoiseTransformer{Percentage: 0.10},
		&DateShiftTransformer{MaxDaysShift: 30},
	}

	for _, t := range builtins {
		if err := r.Register(t); err != nil {
			return err
		}
	}
	return nil
}
