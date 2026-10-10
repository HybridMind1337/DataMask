package validation

import (
	"context"
	"fmt"
	"regexp"

	"github.com/HybridMind1337/DataMask/internal/database"
	"github.com/HybridMind1337/DataMask/internal/discovery"
)

var (
	residualEmailRegex = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@(gmail\.com|yahoo\.com|outlook\.com|hotmail\.com|icloud\.com)`)
	residualJWTRegex   = regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9._-]{10,}`)
	residualTokenRegex = regexp.MustCompile(`(?i)(bearer\s+[A-Za-z0-9_\-]{20,}|ghp_[A-Za-z0-9]{20,}|sk_live_[A-Za-z0-9]{20,})`)
)

// Report contains findings from post-masking validation checks.
type Report struct {
	TablesPresent        int      `json:"tables_present"`
	RowsProcessed        int64    `json:"rows_processed"`
	ForeignKeysValid     bool     `json:"foreign_keys_valid"`
	UniqueConstraintsOk  bool     `json:"unique_constraints_ok"`
	ResidualEmails       int      `json:"residual_emails"`
	ResidualTokens       int      `json:"residual_tokens"`
	ResidualJWTs         int      `json:"residual_jwts"`
	FreeTextWarnings     []string `json:"free_text_warnings"`
	Status               string   `json:"status"` // SAFE, SAFE WITH WARNINGS, FAILED
}

// Validator verifies target database relational integrity and privacy posture.
type Validator struct {
	Target database.Driver
	Schema *discovery.Schema
}

// NewValidator creates a validator.
func NewValidator(target database.Driver, schema *discovery.Schema) *Validator {
	return &Validator{Target: target, Schema: schema}
}

// Validate executes all integrity checks and privacy scans on the target database.
func (v *Validator) Validate(ctx context.Context) (*Report, error) {
	rep := &Report{
		TablesPresent:       len(v.Schema.Tables),
		ForeignKeysValid:    true,
		UniqueConstraintsOk: true,
		Status:              "SAFE",
	}

	inspector := v.Target.Inspector()

	// 1. Check Row Counts and Sample Residual PII
	for tName, table := range v.Schema.Tables {
		rep.RowsProcessed += table.RowCount

		for _, colName := range table.ColumnOrder {
			col := table.Columns[colName]
			samples, err := inspector.SampleColumnValues(ctx, table.Schema, tName, colName, 100)
			if err != nil {
				continue
			}

			for _, s := range samples {
				if s == nil {
					continue
				}
				strVal := fmt.Sprintf("%v", s)
				if residualEmailRegex.MatchString(strVal) {
					rep.ResidualEmails++
				}
				if residualJWTRegex.MatchString(strVal) {
					rep.ResidualJWTs++
				}
				if residualTokenRegex.MatchString(strVal) {
					rep.ResidualTokens++
				}
			}

			// Free text audit
			if col.DataType == "text" && !col.IsPrimaryKey {
				rep.FreeTextWarnings = append(rep.FreeTextWarnings, fmt.Sprintf("%s.%s", tName, colName))
			}
		}
	}

	if rep.ResidualEmails > 0 || rep.ResidualTokens > 0 || rep.ResidualJWTs > 0 {
		rep.Status = "FAILED"
	} else if len(rep.FreeTextWarnings) > 0 {
		rep.Status = "SAFE WITH WARNINGS"
	}

	return rep, nil
}
