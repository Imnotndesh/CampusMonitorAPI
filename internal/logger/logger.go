// internal/logger/logger.go

package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
	FATAL
)

type Mode int

const (
	MINIMAL Mode = iota
	NORMAL
	FULL
)

var (
	levelNames = map[Level]string{
		DEBUG: "DEBUG",
		INFO:  "INFO",
		WARN:  "WARN",
		ERROR: "ERROR",
		FATAL: "FATAL",
	}

	levelColors = map[Level]string{
		DEBUG: "\033[36m",
		INFO:  "\033[32m",
		WARN:  "\033[33m",
		ERROR: "\033[31m",
		FATAL: "\033[35m",
	}

	resetColor = "\033[0m"
)

type Logger struct {
	level      Level
	mode       Mode
	mu         sync.Mutex
	consoleOut io.Writer
	fileOut    io.Writer
	logFile    *os.File
	useColors  bool
}

type Config struct {
	Level       Level
	Mode        Mode
	LogFilePath string
	UseColors   bool
}

func New(cfg Config) (*Logger, error) {
	logger := &Logger{
		level:      cfg.Level,
		mode:       cfg.Mode,
		consoleOut: os.Stdout,
		useColors:  cfg.UseColors,
	}

	if cfg.LogFilePath != "" {
		if err := logger.setupLogFile(cfg.LogFilePath); err != nil {
			return nil, fmt.Errorf("failed to setup log file: %w", err)
		}
	}

	return logger, nil
}

func (l *Logger) setupLogFile(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	l.logFile = file
	l.fileOut = file
	return nil
}

func (l *Logger) Close() error {
	if l.logFile != nil {
		return l.logFile.Close()
	}
	return nil
}

func (l *Logger) log(level Level, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)

	var consoleMsg, fileMsg string

	switch l.mode {
	case MINIMAL:
		consoleMsg = l.formatMinimal(level, message)
		fileMsg = l.formatMinimalFile(level, timestamp, message)

	case NORMAL:
		consoleMsg = l.formatNormal(level, timestamp, message)
		fileMsg = l.formatNormalFile(level, timestamp, message)

	case FULL:
		file, line := l.getCaller()
		consoleMsg = l.formatFull(level, timestamp, file, line, message)
		fileMsg = l.formatFullFile(level, timestamp, file, line, message)
	}

	if l.consoleOut != nil {
		fmt.Fprintln(l.consoleOut, consoleMsg)
	}

	if l.fileOut != nil {
		fmt.Fprintln(l.fileOut, fileMsg)
	}

	if level == FATAL {
		os.Exit(1)
	}
}

func (l *Logger) formatMinimal(level Level, msg string) string {
	levelStr := levelNames[level]
	if l.useColors {
		color := levelColors[level]
		return fmt.Sprintf("%s[%s]%s %s", color, levelStr, resetColor, msg)
	}
	return fmt.Sprintf("[%s] %s", levelStr, msg)
}

func (l *Logger) formatMinimalFile(level Level, timestamp, msg string) string {
	return fmt.Sprintf("%s [%s] %s", timestamp, levelNames[level], msg)
}

func (l *Logger) formatNormal(level Level, timestamp, msg string) string {
	levelStr := levelNames[level]
	if l.useColors {
		color := levelColors[level]
		return fmt.Sprintf("%s[%s]%s %s | %s", color, levelStr, resetColor, timestamp, msg)
	}
	return fmt.Sprintf("[%s] %s | %s", levelStr, timestamp, msg)
}

func (l *Logger) formatNormalFile(level Level, timestamp, msg string) string {
	return fmt.Sprintf("%s [%s] %s", timestamp, levelNames[level], msg)
}

func (l *Logger) formatFull(level Level, timestamp, file string, line int, msg string) string {
	levelStr := levelNames[level]
	location := fmt.Sprintf("%s:%d", file, line)

	if l.useColors {
		color := levelColors[level]
		return fmt.Sprintf("%s[%s]%s %s | %s | %s",
			color, levelStr, resetColor, timestamp, location, msg)
	}
	return fmt.Sprintf("[%s] %s | %s | %s", levelStr, timestamp, location, msg)
}

func (l *Logger) formatFullFile(level Level, timestamp, file string, line int, msg string) string {
	location := fmt.Sprintf("%s:%d", file, line)
	return fmt.Sprintf("%s [%s] %s | %s", timestamp, levelNames[level], location, msg)
}

func (l *Logger) getCaller() (string, int) {
	_, file, line, ok := runtime.Caller(3)
	if !ok {
		return "unknown", 0
	}
	return filepath.Base(file), line
}

func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(FATAL, format, args...)
}

func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *Logger) SetMode(mode Mode) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.mode = mode
}

func ParseLevel(s string) Level {
	switch s {
	case "debug", "DEBUG":
		return DEBUG
	case "info", "INFO":
		return INFO
	case "warn", "WARN", "warning", "WARNING":
		return WARN
	case "error", "ERROR":
		return ERROR
	case "fatal", "FATAL":
		return FATAL
	default:
		return INFO
	}
}

func ParseMode(s string) Mode {
	switch s {
	case "minimal", "MINIMAL":
		return MINIMAL
	case "normal", "NORMAL":
		return NORMAL
	case "full", "FULL":
		return FULL
	default:
		return NORMAL
	}
}

var defaultLogger *Logger

func init() {
	defaultLogger, _ = New(Config{
		Level:     INFO,
		Mode:      NORMAL,
		UseColors: true,
	})
}

func Debug(format string, args ...interface{}) {
	defaultLogger.Debug(format, args...)
}

func Info(format string, args ...interface{}) {
	defaultLogger.Info(format, args...)
}

func Warn(format string, args ...interface{}) {
	defaultLogger.Warn(format, args...)
}

func Error(format string, args ...interface{}) {
	defaultLogger.Error(format, args...)
}

func Fatal(format string, args ...interface{}) {
	defaultLogger.Fatal(format, args...)
}

func SetLevel(level Level) {
	defaultLogger.SetLevel(level)
}

func SetMode(mode Mode) {
	defaultLogger.SetMode(mode)
}

func Close() error {
	return defaultLogger.Close()
}
