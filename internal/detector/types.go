package detector

// SensitivityLevel denotes the privacy impact of a column.
type SensitivityLevel string

const (
	LevelCritical SensitivityLevel = "CRITICAL" // Passwords, API tokens, JWTs, private keys
	LevelHigh     SensitivityLevel = "HIGH"     // Emails, phones, addresses, national IDs
	LevelMedium   SensitivityLevel = "MEDIUM"   // Names, usernames, IP addresses, dates of birth
	LevelLow      SensitivityLevel = "LOW"      // Locales, preferences, flags
	LevelNone     SensitivityLevel = "NONE"     // Non-sensitive operational data
)

// Classification represents the outcome of detecting sensitive properties.
type Classification struct {
	TableName       string           `json:"table_name" yaml:"table_name"`
	ColumnName      string           `json:"column_name" yaml:"column_name"`
	DataType        string           `json:"data_type" yaml:"data_type"`
	DetectedType    string           `json:"detected_type" yaml:"detected_type"` // e.g. "email", "phone", "password"
	Confidence      float64          `json:"confidence" yaml:"confidence"`       // 0.0 to 1.0
	Level           SensitivityLevel `json:"level" yaml:"level"`
	RecommendedRule string           `json:"recommended_rule" yaml:"recommended_rule"`
	IsFreeText      bool             `json:"is_free_text" yaml:"is_free_text"`
}
