package internal

import (
	"fmt"
	"os"
)

// ANSI color codes.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[0;31m"
	colorGreen  = "\033[0;32m"
	colorYellow = "\033[0;33m"
	colorCyan   = "\033[0;36m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

// Say prints an info message.
func Say(format string, args ...interface{}) {
	fmt.Fprintf(os.Stdout, colorGreen+"og"+colorReset+": "+format+"\n", args...)
}

// Warn prints a warning message.
func Warn(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, colorYellow+"warn"+colorReset+": "+format+"\n", args...)
}

// Die prints an error and exits with code 1.
func Die(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, colorRed+"error"+colorReset+": "+format+"\n", args...)
	os.Exit(1)
}

// Dim prints a dim message.
func Dim(format string, args ...interface{}) {
	fmt.Fprintf(os.Stdout, colorDim+format+colorReset+"\n", args...)
}

// Color accessors for use by cmd package.
func ColorReset() string  { return colorReset }
func ColorRed() string    { return colorRed }
func ColorGreen() string  { return colorGreen }
func ColorYellow() string { return colorYellow }
func ColorCyan() string   { return colorCyan }
func ColorBold() string   { return colorBold }
func ColorDim() string    { return colorDim }
