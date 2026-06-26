package logger

import (
	"fmt"
	"os"
)

// ANSI Color Constants
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[91m"
	ColorGreen  = "\033[92m"
	ColorYellow = "\033[93m"
	ColorCyan   = "\033[96m"
)

// Logger defines the common interface for logging across the application.
type Logger interface {
	Info(format string, a ...interface{})
	Warn(format string, a ...interface{})
	Error(format string, a ...interface{})
	Success(format string, a ...interface{})
}

type consoleLogger struct{}

// Log is the global logger instance.
var Log Logger = &consoleLogger{}

// safeLog executes logging operations within a panic recovery boundary.
func safeLog(logFunc func()) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "[Logger Panic Recovery] 로깅 실행 도중 패닉 감지 및 자동 복구됨: %v\n", r)
		}
	}()
	logFunc()
}

func (l *consoleLogger) Info(format string, a ...interface{}) {
	safeLog(func() {
		_, err := fmt.Printf(format, a...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[Logger Error] Info 로깅 쓰기 실패: %v\n", err)
		}
	})
}

func (l *consoleLogger) Warn(format string, a ...interface{}) {
	safeLog(func() {
		msg := fmt.Sprintf(format, a...)
		_, err := fmt.Fprint(os.Stderr, ColorYellow+msg+ColorReset)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[Logger Error] Warn 로깅 쓰기 실패: %v\n", err)
		}
	})
}

func (l *consoleLogger) Error(format string, a ...interface{}) {
	safeLog(func() {
		msg := fmt.Sprintf(format, a...)
		_, err := fmt.Fprint(os.Stderr, ColorRed+msg+ColorReset)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[Logger Error] Error 로깅 쓰기 실패: %v\n", err)
		}
	})
}

func (l *consoleLogger) Success(format string, a ...interface{}) {
	safeLog(func() {
		msg := fmt.Sprintf(format, a...)
		_, err := fmt.Fprint(os.Stdout, ColorGreen+msg+ColorReset)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[Logger Error] Success 로깅 쓰기 실패: %v\n", err)
		}
	})
}
