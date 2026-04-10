package applog

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Level controls which messages are emitted.
type Level int

const (
	// LevelQuiet suppresses informational and debug output (errors still print via returned errors).
	LevelQuiet Level = iota
	// LevelInfo is the default: high-level progress messages.
	LevelInfo
	// LevelDebug adds per-request detail.
	LevelDebug
)

// ParseLevel returns the level for flag values such as "quiet", "info", "debug".
func ParseLevel(s string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "quiet", "error", "0":
		return LevelQuiet, nil
	case "info", "1":
		return LevelInfo, nil
	case "debug", "trace", "verbose", "2":
		return LevelDebug, nil
	default:
		return 0, fmt.Errorf("invalid log level %q (want quiet, info, or debug)", s)
	}
}

// Logger writes leveled messages to Out (typically os.Stderr).
type Logger struct {
	Out   io.Writer
	Level Level
}

// New returns a logger writing to stderr at the given level.
func New(level Level) *Logger {
	return &Logger{Out: os.Stderr, Level: level}
}

// Discard returns a logger that never prints.
func Discard() *Logger {
	return &Logger{Out: io.Discard, Level: LevelQuiet}
}

// Infof prints a message when level is Info or Debug.
func (l *Logger) Infof(format string, args ...interface{}) {
	if l == nil || l.Level < LevelInfo {
		return
	}
	fmt.Fprintf(l.Out, format+"\n", args...)
}

// Debugf prints a message only at Debug.
func (l *Logger) Debugf(format string, args ...interface{}) {
	if l == nil || l.Level < LevelDebug {
		return
	}
	fmt.Fprintf(l.Out, "debug: "+format+"\n", args...)
}
