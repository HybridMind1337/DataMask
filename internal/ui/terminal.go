package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// ANSI Color and style sequences
const (
	Reset     = "\033[0m"
	Bold      = "\033[1m"
	Dim       = "\033[2m"
	Italic    = "\033[3m"
	Underline = "\033[4m"

	// Foreground colors
	FgBlack   = "\033[30m"
	FgRed     = "\033[31m"
	FgGreen   = "\033[32m"
	FgYellow  = "\033[33m"
	FgBlue    = "\033[34m"
	FgMagenta = "\033[35m"
	FgCyan    = "\033[36m"
	FgWhite   = "\033[37m"
	FgGray    = "\033[90m"

	// High intensity colors
	FgHiRed     = "\033[91m"
	FgHiGreen   = "\033[92m"
	FgHiYellow  = "\033[93m"
	FgHiBlue    = "\033[94m"
	FgHiMagenta = "\033[95m"
	FgHiCyan    = "\033[96m"
	FgHiWhite   = "\033[97m"

	// Background colors
	BgRed     = "\033[41m"
	BgGreen   = "\033[42m"
	BgYellow  = "\033[43m"
	BgBlue    = "\033[44m"
	BgMagenta = "\033[45m"
	BgCyan    = "\033[46m"
)

// Symbols for clean terminal UI
const (
	SymbolCheck = "✓"
	SymbolCross = "✗"
	SymbolWarn  = "⚠"
	SymbolArrow = "→"
	SymbolBullet= "•"
	SymbolLock  = "🔒"
	SymbolShield= "🛡 "
)

// Banner renders a clean, modern CLI header.
func Banner(w io.Writer, subtitle string) {
	fmt.Fprintf(w, "%s%sDataMask%s %s%s%s\n", Bold, FgHiCyan, Reset, Dim, "v0.1.0", Reset)
	if subtitle != "" {
		fmt.Fprintf(w, "%s%s%s\n", Dim, subtitle, Reset)
	}
	fmt.Fprintf(w, "%s%s%s\n", FgGray, strings.Repeat("─", 50), Reset)
}

// Badge renders a colored status pill like [CRITICAL], [HIGH], [SAFE].
func Badge(level string) string {
	switch strings.ToUpper(level) {
	case "CRITICAL":
		return fmt.Sprintf("%s%s CRITICAL %s", Bold, BgRed+FgHiWhite, Reset)
	case "HIGH":
		return fmt.Sprintf("%s%s HIGH %s", Bold, FgHiRed, Reset)
	case "MEDIUM":
		return fmt.Sprintf("%s%s MEDIUM %s", Bold, FgHiYellow, Reset)
	case "LOW":
		return fmt.Sprintf("%s%s LOW %s", Bold, FgCyan, Reset)
	case "SAFE":
		return fmt.Sprintf("%s%s SAFE %s", Bold, BgGreen+FgHiWhite, Reset)
	case "PRESERVE":
		return fmt.Sprintf("%s%s PRESERVE %s", Dim, FgGray, Reset)
	default:
		return fmt.Sprintf("[%s]", level)
	}
}

// Success prints a green checked item.
func Success(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("%s%s%s %s\n", Bold, FgHiGreen, SymbolCheck, msg)
}

// Warning prints a yellow warning item.
func Warning(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("%s%s%s %s\n", Bold, FgHiYellow, SymbolWarn, msg)
}

// Error prints a red failure item.
func Error(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("%s%s%s %s\n", Bold, FgHiRed, SymbolCross, msg)
}

// Info prints a muted cyan informational bullet.
func Info(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("%s%s%s %s\n", Bold, FgHiCyan, SymbolArrow, msg)
}

// KeyValue prints aligned metadata.
func KeyValue(key string, val any) {
	fmt.Printf("%s%-20s%s %v\n", Dim, key+":", Reset, val)
}

// ProgressBar renders a high-res fractional Unicode progress bar.
// Example: ▰▰▰▰▰▰▰▰▰▰▰▰▱▱▱▱▱▱▱▱ 60.0%
func ProgressBar(current, total int64, width int) string {
	if total <= 0 {
		return strings.Repeat("▱", width)
	}
	ratio := float64(current) / float64(total)
	if ratio > 1.0 {
		ratio = 1.0
	}
	filled := int(ratio * float64(width))
	if filled > width {
		filled = width
	}
	empty := width - filled
	if empty < 0 {
		empty = 0
	}

	return fmt.Sprintf("%s%s%s%s%s",
		FgHiCyan,
		strings.Repeat("▰", filled),
		FgGray,
		strings.Repeat("▱", empty),
		Reset,
	)
}

// StreamProgress renders an animated in-place line for table streaming.
func StreamProgress(table string, done, total int64, rate float64, elapsed time.Duration) {
	pct := 0.0
	if total > 0 {
		pct = (float64(done) / float64(total)) * 100.0
	}
	bar := ProgressBar(done, total, 16)
	fmt.Printf("\r  %s%-16s%s %s %s%5.1f%%%s  %s%d/%d%s  %s(%.0f rows/s)%s",
		Bold+FgHiWhite, table, Reset,
		bar,
		Bold+FgCyan, pct, Reset,
		Dim, done, total, Reset,
		FgGreen, rate, Reset,
	)
	_ = os.Stdout.Sync()
}

// SectionHeader prints a modern bordered section title.
func SectionHeader(title string) {
	fmt.Printf("\n%s%s %s %s%s\n", Bold, FgHiBlue, strings.ToUpper(title), Reset, FgGray)
}
