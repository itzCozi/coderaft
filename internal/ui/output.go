// Package ui provides consistent CLI output formatting.
//
// All user-facing terminal output should go through these helpers
// to guarantee a uniform style modeled after git's output:
//   - ASCII-only (no emojis, no Unicode box-drawing)
//   - Terse, lowercase prefixes
//   - Consistent 2-space indent for sub-items
package ui

import "fmt"

// Status prints a progress/action line: "  project: pulling image..."
func Status(msg string, args ...interface{}) {
	fmt.Printf(msg+"\n", args...)
}

// Statusf is an alias for Status kept for readability at call-sites
// that always use format args.
func Statusf(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

// Step prints a numbered step: "  [1/3] installing packages"
func Step(current, total int, msg string, args ...interface{}) {
	prefix := fmt.Sprintf("[%d/%d] ", current, total)
	fmt.Printf(prefix+msg+"\n", args...)
}

// Success prefixes with "done: " — used for final completion messages.
func Success(msg string, args ...interface{}) {
	fmt.Printf("done: "+msg+"\n", args...)
}

// Warning prefixes with "warning: ".
func Warning(msg string, args ...interface{}) {
	fmt.Printf("warning: "+msg+"\n", args...)
}

// Error prefixes with "error: " — for non-fatal error reporting to the
// user (fatal errors should be returned as Go errors).
func Error(msg string, args ...interface{}) {
	fmt.Printf("error: "+msg+"\n", args...)
}

// Info prints a plain informational line (no prefix).
func Info(msg string, args ...interface{}) {
	fmt.Printf(msg+"\n", args...)
}

// Detail prints an indented sub-item: "  key: value".
func Detail(key, value string) {
	fmt.Printf("  %s: %s\n", key, value)
}

// Item prints a bulleted list item: "  - text".
func Item(msg string, args ...interface{}) {
	fmt.Printf("  - "+msg+"\n", args...)
}

// Header prints a section header.
func Header(msg string, args ...interface{}) {
	fmt.Printf(msg+"\n", args...)
}

// Blank prints an empty line for visual separation.
func Blank() {
	fmt.Println()
}

// Prompt prints a prompt string without a trailing newline.
func Prompt(msg string, args ...interface{}) {
	fmt.Printf(msg, args...)
}

// Summary prints a "summary: N done, M failed" line.
func Summary(label string, counts ...interface{}) {
	fmt.Printf("summary: "+label+"\n", counts...)
}
