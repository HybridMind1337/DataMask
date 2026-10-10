package reporting

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/HybridMind1337/DataMask/internal/validation"
)

// AuditReport models the final summary artifact of a DataMask execution.
type AuditReport struct {
	RunID           string             `json:"run_id"`
	Version         string             `json:"version"`
	StartedAt       time.Time          `json:"started_at"`
	CompletedAt     time.Time          `json:"completed_at"`
	DurationSec     float64            `json:"duration_sec"`
	SourceDriver    string             `json:"source_driver"`
	TargetDriver    string             `json:"target_driver"`
	TablesProcessed int                `json:"tables_processed"`
	RowsProcessed   int64              `json:"rows_processed"`
	FieldsMasked    int64              `json:"fields_masked"`
	Validation      *validation.Report `json:"validation,omitempty"`
	Status          string             `json:"status"`
}

// GenerateRunID produces a unique audit run identifier (e.g. DM-2026-10-10-8F92A1).
func GenerateRunID() string {
	now := time.Now().UTC()
	return fmt.Sprintf("DM-%04d-%02d-%02d-%06X",
		now.Year(), now.Month(), now.Day(), now.UnixNano()%0xFFFFFF)
}

// WriteJSON formats the report as indented JSON.
func (r *AuditReport) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// WriteConsole formats the report for terminal output with modern executive styling.
func (r *AuditReport) WriteConsole(w io.Writer) {
	fmt.Fprintf(w, "\n\033[1m\033[96mDataMask Audit Report\033[0m\n")
	fmt.Fprintf(w, "\033[90m%s\033[0m\n", "──────────────────────────────────────────────────")
	fmt.Fprintf(w, "\033[2m%-20s\033[0m \033[1m%s\033[0m\n", "Run ID:", r.RunID)
	
	statusBadge := "\033[32m\033[1m✓ SUCCESS\033[0m"
	if r.Status != "SUCCESS" {
		statusBadge = "\033[31m\033[1m✗ " + r.Status + "\033[0m"
	}
	fmt.Fprintf(w, "\033[2m%-20s\033[0m %s\n", "Status:", statusBadge)
	fmt.Fprintf(w, "\033[2m%-20s\033[0m %d\n", "Tables Processed:", r.TablesProcessed)
	fmt.Fprintf(w, "\033[2m%-20s\033[0m %d\n", "Rows Processed:", r.RowsProcessed)
	fmt.Fprintf(w, "\033[2m%-20s\033[0m \033[36m%d\033[0m\n", "Fields Masked:", r.FieldsMasked)
	fmt.Fprintf(w, "\033[2m%-20s\033[0m %.2fs\n", "Duration:", r.DurationSec)

	if r.Validation != nil {
		fmt.Fprintf(w, "\n\033[1mValidation Posture:\033[0m\n")
		fkBadge := "\033[32m✓ Valid\033[0m"
		if !r.Validation.ForeignKeysValid {
			fkBadge = "\033[31m✗ Broken\033[0m"
		}
		fmt.Fprintf(w, "  • Foreign Keys:     %s\n", fkBadge)
		fmt.Fprintf(w, "  • Residual Emails:  \033[1m%d\033[0m\n", r.Validation.ResidualEmails)
		fmt.Fprintf(w, "  • Residual Tokens:  \033[1m%d\033[0m\n", r.Validation.ResidualTokens)
		if len(r.Validation.FreeTextWarnings) > 0 {
			fmt.Fprintf(w, "  • \033[33mFree-text alerts: %d columns\033[0m\n", len(r.Validation.FreeTextWarnings))
		}
	}
	fmt.Fprintf(w, "\033[90m%s\033[0m\n", "──────────────────────────────────────────────────")
	fmt.Fprintf(w, "\033[2mSource database remained strictly read-only.\033[0m\n\n")
}
