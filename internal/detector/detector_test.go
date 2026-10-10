package detector_test

import (
	"context"
	"testing"

	"github.com/HybridMind1337/DataMask/internal/detector"
	"github.com/HybridMind1337/DataMask/internal/discovery"
)

func TestDetectorClassification(t *testing.T) {
	det := detector.NewDetector()
	ctx := context.Background()

	tests := []struct {
		column   string
		dataType string
		expected detector.SensitivityLevel
		rule     string
	}{
		{"password_hash", "varchar(255)", detector.LevelCritical, "dev_auth"},
		{"api_token", "text", detector.LevelCritical, "redact"},
		{"email", "varchar(255)", detector.LevelHigh, "fake_email"},
		{"phone_number", "varchar(50)", detector.LevelHigh, "fake_phone"},
		{"first_name", "varchar(100)", detector.LevelMedium, "fake_first_name"},
		{"ip_address", "varchar(45)", detector.LevelMedium, "fake_ipv4"},
		{"support_notes", "text", detector.LevelMedium, "pattern_scrub"},
		{"order_total", "decimal(10,2)", detector.LevelNone, "preserve"},
	}

	for _, tt := range tests {
		col := &discovery.Column{
			Name:     tt.column,
			DataType: tt.dataType,
		}
		cls := det.ClassifyColumn(ctx, "test_table", tt.column, col)

		if cls.Level != tt.expected {
			t.Errorf("column %q expected level %s, got %s", tt.column, tt.expected, cls.Level)
		}
		if cls.RecommendedRule != tt.rule {
			t.Errorf("column %q expected rule %s, got %s", tt.column, tt.rule, cls.RecommendedRule)
		}
	}
}
