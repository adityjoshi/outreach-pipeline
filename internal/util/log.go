package util

import (
	"fmt"
	"log"
	"os"
)

var logger = log.New(os.Stderr, "", log.LstdFlags)

// Logf writes a formatted pipeline progress line to stderr.
func Logf(format string, args ...any) {
	logger.Printf(format, args...)
}

// LogStage prints a stage banner.
func LogStage(n int, name string) {
	Logf("")
	Logf("=== Stage %d: %s ===", n, name)
}

// LogSummary prints a compact table row.
func LogSummary(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}
