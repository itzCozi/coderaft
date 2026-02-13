package ui

import "fmt"

// Verbose controls whether intermediate progress messages are shown.
// Set via the --verbose global flag.
var Verbose bool

func Status(msg string, args ...interface{}) {
	if !Verbose {
		return
	}
	fmt.Printf(msg+"\n", args...)
}

func Statusf(format string, args ...interface{}) {
	if !Verbose {
		return
	}
	fmt.Printf(format+"\n", args...)
}

func Step(current, total int, msg string, args ...interface{}) {
	prefix := fmt.Sprintf("[%d/%d] ", current, total)
	fmt.Printf(prefix+msg+"\n", args...)
}

func Success(msg string, args ...interface{}) {
	fmt.Printf("done: "+msg+"\n", args...)
}

func Warning(msg string, args ...interface{}) {
	fmt.Printf("warning: "+msg+"\n", args...)
}

func Error(msg string, args ...interface{}) {
	fmt.Printf("error: "+msg+"\n", args...)
}

func Info(msg string, args ...interface{}) {
	fmt.Printf(msg+"\n", args...)
}

func Detail(key, value string) {
	fmt.Printf("  %s: %s\n", key, value)
}

func Item(msg string, args ...interface{}) {
	fmt.Printf("  - "+msg+"\n", args...)
}

func Header(msg string, args ...interface{}) {
	fmt.Printf(msg+"\n", args...)
}

func Blank() {
	fmt.Println()
}

func Prompt(msg string, args ...interface{}) {
	fmt.Printf("? "+msg, args...)
}

func Summary(label string, counts ...interface{}) {
	fmt.Printf("summary: "+label+"\n", counts...)
}
