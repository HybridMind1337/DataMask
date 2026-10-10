package detector

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/HybridMind1337/DataMask/internal/discovery"
)

var (
	// Critical patterns
	passwordPattern = regexp.MustCompile(`(?i)(password|passwd|pass_hash|pwhash|secret|token|api_key|access_token|refresh_token|private_key|auth_token)`)
	// High patterns
	emailPattern    = regexp.MustCompile(`(?i)(email|e_mail|mail_address)`)
	phonePattern    = regexp.MustCompile(`(?i)(phone|telephone|mobile|cell|fax)`)
	addressPattern  = regexp.MustCompile(`(?i)(street|address|addr_line|postcode|postal_code|zip_code|zipcode)`)
	cardPattern     = regexp.MustCompile(`(?i)(credit_card|card_num|card_number|cvv|cvc|pan)`)
	ssnPattern      = regexp.MustCompile(`(?i)(ssn|social_security|national_id|id_number|tax_id)`)
	// Medium patterns
	namePattern     = regexp.MustCompile(`(?i)(first_name|firstname|last_name|lastname|full_name|fullname|surname|display_name|user_name|username)`)
	dobPattern      = regexp.MustCompile(`(?i)(birth_date|dob|birthdate|date_of_birth)`)
	ipPattern       = regexp.MustCompile(`(?i)(ip_address|ip_addr|client_ip|remote_ip|ipv4|ipv6)`)
	// Free text risk
	freeTextPattern = regexp.MustCompile(`(?i)(message|comment|notes|description|body|log|feedback|content|details)`)
)

// Detector performs multi-signal sensitive field detection.
type Detector struct{}

// NewDetector creates an instance of the detection engine.
func NewDetector() *Detector {
	return &Detector{}
}

// ClassifyColumn inspects column name and metadata to determine sensitivity.
func (d *Detector) ClassifyColumn(ctx context.Context, table, column string, col *discovery.Column) Classification {
	colName := strings.ToLower(column)
	dataType := strings.ToLower(col.DataType)

	// Free-text detection
	isFreeText := freeTextPattern.MatchString(colName) || strings.Contains(dataType, "text") || strings.Contains(dataType, "clob")

	// JSON / JSONB deep masking detection
	if strings.Contains(dataType, "json") || strings.Contains(col.RawType, "json") {
		return Classification{
			TableName:       table,
			ColumnName:      column,
			DataType:        col.DataType,
			DetectedType:    "json_payload",
			Confidence:      0.90,
			Level:           LevelMedium,
			RecommendedRule: "json_mask",
		}
	}

	// 1. Critical fields
	if passwordPattern.MatchString(colName) {
		rec := "redact"
		if strings.Contains(colName, "pass") {
			rec = "dev_auth"
		}
		return Classification{
			TableName:       table,
			ColumnName:      column,
			DataType:        col.DataType,
			DetectedType:    "credential",
			Confidence:      0.98,
			Level:           LevelCritical,
			RecommendedRule: rec,
		}
	}

	// 2. High risk PII
	if emailPattern.MatchString(colName) {
		return Classification{
			TableName:       table,
			ColumnName:      column,
			DataType:        col.DataType,
			DetectedType:    "email",
			Confidence:      0.95,
			Level:           LevelHigh,
			RecommendedRule: "fake_email",
		}
	}
	if phonePattern.MatchString(colName) {
		return Classification{
			TableName:       table,
			ColumnName:      column,
			DataType:        col.DataType,
			DetectedType:    "phone",
			Confidence:      0.92,
			Level:           LevelHigh,
			RecommendedRule: "fake_phone",
		}
	}
	if ipPattern.MatchString(colName) {
		rec := "fake_ipv4"
		if strings.Contains(colName, "v6") {
			rec = "fake_ipv6"
		}
		return Classification{
			TableName:       table,
			ColumnName:      column,
			DataType:        col.DataType,
			DetectedType:    "ip_address",
			Confidence:      0.90,
			Level:           LevelMedium,
			RecommendedRule: rec,
		}
	}
	if addressPattern.MatchString(colName) {
		rec := "fake_address"
		if strings.Contains(colName, "post") || strings.Contains(colName, "zip") {
			rec = "fake_postcode"
		}
		return Classification{
			TableName:       table,
			ColumnName:      column,
			DataType:        col.DataType,
			DetectedType:    "address",
			Confidence:      0.88,
			Level:           LevelHigh,
			RecommendedRule: rec,
		}
	}
	if cardPattern.MatchString(colName) {
		return Classification{
			TableName:       table,
			ColumnName:      column,
			DataType:        col.DataType,
			DetectedType:    "payment_card",
			Confidence:      0.96,
			Level:           LevelCritical,
			RecommendedRule: "redact",
		}
	}
	if ssnPattern.MatchString(colName) {
		return Classification{
			TableName:       table,
			ColumnName:      column,
			DataType:        col.DataType,
			DetectedType:    "national_id",
			Confidence:      0.94,
			Level:           LevelHigh,
			RecommendedRule: "redact",
		}
	}

	// 3. Medium risk fields
	if namePattern.MatchString(colName) {
		rec := "fake_name"
		if strings.Contains(colName, "first") {
			rec = "fake_first_name"
		} else if strings.Contains(colName, "last") || strings.Contains(colName, "surname") {
			rec = "fake_last_name"
		} else if strings.Contains(colName, "user") {
			rec = "fake_username"
		}
		return Classification{
			TableName:       table,
			ColumnName:      column,
			DataType:        col.DataType,
			DetectedType:    "person_name",
			Confidence:      0.85,
			Level:           LevelMedium,
			RecommendedRule: rec,
		}
	}
	if dobPattern.MatchString(colName) {
		return Classification{
			TableName:       table,
			ColumnName:      column,
			DataType:        col.DataType,
			DetectedType:    "birth_date",
			Confidence:      0.87,
			Level:           LevelMedium,
			RecommendedRule: "random_date",
		}
	}
	if ipPattern.MatchString(colName) {
		rec := "fake_ipv4"
		if strings.Contains(colName, "v6") {
			rec = "fake_ipv6"
		}
		return Classification{
			TableName:       table,
			ColumnName:      column,
			DataType:        col.DataType,
			DetectedType:    "ip_address",
			Confidence:      0.90,
			Level:           LevelMedium,
			RecommendedRule: rec,
		}
	}

	// 4. Free-text fields
	if isFreeText && !col.IsPrimaryKey && !col.IsForeignKey {
		return Classification{
			TableName:       table,
			ColumnName:      column,
			DataType:        col.DataType,
			DetectedType:    "free_text_risk",
			Confidence:      0.70,
			Level:           LevelMedium,
			RecommendedRule: "pattern_scrub",
			IsFreeText:      true,
		}
	}

	// Default operational field
	return Classification{
		TableName:       table,
		ColumnName:      column,
		DataType:        col.DataType,
		DetectedType:    "operational",
		Confidence:      0.10,
		Level:           LevelNone,
		RecommendedRule: "preserve",
	}
}

// ScanSchema performs full classification across all tables and columns.
func (d *Detector) ScanSchema(ctx context.Context, schema *discovery.Schema) []Classification {
	var results []Classification
	for tableName, table := range schema.Tables {
		for _, colName := range table.ColumnOrder {
			col := table.Columns[colName]
			cls := d.ClassifyColumn(ctx, tableName, colName, col)
			results = append(results, cls)
		}
	}
	return results
}

// SampleMatchesValueRegex tests safe string samples for regex patterns.
func SampleMatchesValueRegex(samples []any, pattern *regexp.Regexp) float64 {
	if len(samples) == 0 {
		return 0
	}
	matches := 0
	for _, s := range samples {
		if s == nil {
			continue
		}
		str := fmt.Sprintf("%v", s)
		if pattern.MatchString(str) {
			matches++
		}
	}
	return float64(matches) / float64(len(samples))
}
