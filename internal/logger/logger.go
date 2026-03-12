// Package logger provides unified logging with level-based filtering.
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

// LogLevel represents the logging level.
type LogLevel int

const (
	// DebugLevel logs are typically voluminous, and are usually disabled in production.
	DebugLevel LogLevel = iota
	// InfoLevel is the default logging priority.
	InfoLevel
	// WarnLevel logs are more important than Info, but don't need individual human review.
	WarnLevel
	// ErrorLevel logs are high-priority. If an application is running smoothly, it shouldn't generate any error logs.
	ErrorLevel
	// FatalLevel logs and then calls os.Exit(1).
	FatalLevel
)

// String returns the string representation of the log level.
func (l LogLevel) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	default:
		return fmt.Sprintf("LEVEL(%d)", l)
	}
}

// ParseLevel parses a string to a LogLevel.
func ParseLevel(level string) (LogLevel, error) {
	switch strings.ToLower(level) {
	case "debug":
		return DebugLevel, nil
	case "info":
		return InfoLevel, nil
	case "warn", "warning":
		return WarnLevel, nil
	case "error":
		return ErrorLevel, nil
	case "fatal":
		return FatalLevel, nil
	default:
		return InfoLevel, fmt.Errorf("invalid log level: %s (valid: debug, info, warn, error, fatal)", level)
	}
}

// Logger is a leveled logger.
type Logger struct {
	mu        sync.Mutex
	level     LogLevel
	output    io.Writer
	stdLogger *log.Logger
	fileLine  bool
}

// Default logger instance.
var defaultLogger = &Logger{
	level:     InfoLevel,
	output:    os.Stderr,
	fileLine:  false,
	stdLogger: log.New(os.Stderr, "", log.LstdFlags),
}

// New creates a new Logger with the specified level.
func New(level LogLevel) *Logger {
	l := &Logger{
		level:    level,
		output:   os.Stderr,
		fileLine: level == DebugLevel,
	}
	l.stdLogger = log.New(l.output, "", log.LstdFlags)
	return l
}

// SetLevel sets the logging level.
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
	l.fileLine = level == DebugLevel
}

// SetOutput sets the output destination.
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = w
	l.stdLogger.SetOutput(w)
}

// IsEnabled returns true if logging is enabled for the given level.
func (l *Logger) IsEnabled(level LogLevel) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return level >= l.level
}

// Debug logs a message at DebugLevel.
func (l *Logger) Debug(format string, v ...interface{}) {
	if !l.IsEnabled(DebugLevel) {
		return
	}
	l.log(DebugLevel, format, v...)
}

// Debugf logs a formatted message at DebugLevel.
func (l *Logger) Debugf(format string, v ...interface{}) {
	if !l.IsEnabled(DebugLevel) {
		return
	}
	l.log(DebugLevel, format, v...)
}

// Info logs a message at InfoLevel.
func (l *Logger) Info(format string, v ...interface{}) {
	if !l.IsEnabled(InfoLevel) {
		return
	}
	l.log(InfoLevel, format, v...)
}

// Infof logs a formatted message at InfoLevel.
func (l *Logger) Infof(format string, v ...interface{}) {
	if !l.IsEnabled(InfoLevel) {
		return
	}
	l.log(InfoLevel, format, v...)
}

// Warn logs a message at WarnLevel.
func (l *Logger) Warn(format string, v ...interface{}) {
	if !l.IsEnabled(WarnLevel) {
		return
	}
	l.log(WarnLevel, format, v...)
}

// Warnf logs a formatted message at WarnLevel.
func (l *Logger) Warnf(format string, v ...interface{}) {
	if !l.IsEnabled(WarnLevel) {
		return
	}
	l.log(WarnLevel, format, v...)
}

// Error logs a message at ErrorLevel.
func (l *Logger) Error(format string, v ...interface{}) {
	if !l.IsEnabled(ErrorLevel) {
		return
	}
	l.log(ErrorLevel, format, v...)
}

// Errorf logs a formatted message at ErrorLevel.
func (l *Logger) Errorf(format string, v ...interface{}) {
	if !l.IsEnabled(ErrorLevel) {
		return
	}
	l.log(ErrorLevel, format, v...)
}

// Fatal logs a message at FatalLevel and exits.
func (l *Logger) Fatal(format string, v ...interface{}) {
	l.log(FatalLevel, format, v...)
	os.Exit(1)
}

// Fatalf logs a formatted message at FatalLevel and exits.
func (l *Logger) Fatalf(format string, v ...interface{}) {
	l.log(FatalLevel, format, v...)
	os.Exit(1)
}

// Println logs a message at InfoLevel.
func (l *Logger) Println(v ...interface{}) {
	if !l.IsEnabled(InfoLevel) {
		return
	}
	msg := fmt.Sprint(v...)
	l.log(InfoLevel, "%s", msg)
}

// Printf logs a formatted message at InfoLevel.
func (l *Logger) Printf(format string, v ...interface{}) {
	l.Infof(format, v...)
}

// log is the internal logging method.
func (l *Logger) log(level LogLevel, format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	msg := fmt.Sprintf(format, v...)
	timestamp := l.stdLogger.Prefix() // Get timestamp from stdLogger

	var fullMsg string
	if l.fileLine && level == DebugLevel {
		// Include file and line for debug logs
		fullMsg = fmt.Sprintf("%s[%s] %s", timestamp, level.String(), msg)
	} else {
		fullMsg = fmt.Sprintf("%s[%s] %s", timestamp, level.String(), msg)
	}

	l.stdLogger.Print(fullMsg)
}

// SetLevel sets the default logger's logging level.
func SetLevel(level LogLevel) {
	defaultLogger.SetLevel(level)
}

// SetLevelString parses and sets the default logger's logging level from a string.
func SetLevelString(levelStr string) error {
	level, err := ParseLevel(levelStr)
	if err != nil {
		return err
	}
	SetLevel(level)
	return nil
}

// SetOutput sets the default logger's output destination.
func SetOutput(w io.Writer) {
	defaultLogger.SetOutput(w)
}

// Debug logs a message at DebugLevel using the default logger.
func Debug(format string, v ...interface{}) {
	defaultLogger.Debug(format, v...)
}

// Debugf logs a formatted message at DebugLevel using the default logger.
func Debugf(format string, v ...interface{}) {
	defaultLogger.Debugf(format, v...)
}

// Info logs a message at InfoLevel using the default logger.
func Info(format string, v ...interface{}) {
	defaultLogger.Info(format, v...)
}

// Infof logs a formatted message at InfoLevel using the default logger.
func Infof(format string, v ...interface{}) {
	defaultLogger.Infof(format, v...)
}

// Warn logs a message at WarnLevel using the default logger.
func Warn(format string, v ...interface{}) {
	defaultLogger.Warn(format, v...)
}

// Warnf logs a formatted message at WarnLevel using the default logger.
func Warnf(format string, v ...interface{}) {
	defaultLogger.Warnf(format, v...)
}

// Error logs a message at ErrorLevel using the default logger.
func Error(format string, v ...interface{}) {
	defaultLogger.Error(format, v...)
}

// Errorf logs a formatted message at ErrorLevel using the default logger.
func Errorf(format string, v ...interface{}) {
	defaultLogger.Errorf(format, v...)
}

// Fatal logs a message at FatalLevel and exits using the default logger.
func Fatal(format string, v ...interface{}) {
	defaultLogger.Fatal(format, v...)
}

// Fatalf logs a formatted message at FatalLevel and exits using the default logger.
func Fatalf(format string, v ...interface{}) {
	defaultLogger.Fatalf(format, v...)
}

// Println logs a message at InfoLevel using the default logger.
func Println(v ...interface{}) {
	defaultLogger.Println(v...)
}

// Printf logs a formatted message at InfoLevel using the default logger.
func Printf(format string, v ...interface{}) {
	defaultLogger.Printf(format, v...)
}
