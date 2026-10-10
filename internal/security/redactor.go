package security

import (
	"fmt"
	"regexp"
)

var (
	emailRegex = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`)
	phoneRegex = regexp.MustCompile(`\+?[0-9]{1,3}?[-.\s]?\(?[0-9]{2,4}\)?[-.\s]?[0-9]{3,4}[-.\s]?[0-9]{3,4}`)
	jwtRegex   = regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9._-]{10,}`)
	apiKeyRegex = regexp.MustCompile(`(?i)(api[_-]?key|secret|token|bearer)[=:\s]+[A-Za-z0-9_\-]{16,}`)
)

// ScrubMessage removes potential raw PII values from error messages and log outputs.
func ScrubMessage(msg string) string {
	msg = emailRegex.ReplaceAllString(msg, "[REDACTED_EMAIL]")
	msg = jwtRegex.ReplaceAllString(msg, "[REDACTED_JWT]")
	msg = apiKeyRegex.ReplaceAllString(msg, "$1=[REDACTED_TOKEN]")
	return msg
}

// FormatSafeError formats an operational error without echoing raw data values.
func FormatSafeError(table, column string, rowIdentifier any, err error) error {
	idStr := ""
	if rowIdentifier != nil {
		idStr = fmt.Sprintf(" at identifier %v", rowIdentifier)
	}
	return fmt.Errorf("error processing %s.%s%s: %s", table, column, idStr, ScrubMessage(err.Error()))
}
