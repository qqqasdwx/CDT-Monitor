package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

type Logger struct {
	out   io.Writer
	level Level
	mu    sync.Mutex
}

func New(out io.Writer, level Level) *Logger {
	if out == nil {
		out = os.Stdout
	}
	return &Logger{out: out, level: level}
}

func ParseLevel(value string) Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return LevelDebug
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

func (l *Logger) Debug(message string, attrs ...any) {
	l.write(LevelDebug, "debug", message, attrs...)
}

func (l *Logger) Info(message string, attrs ...any) {
	l.write(LevelInfo, "info", message, attrs...)
}

func (l *Logger) Warn(message string, attrs ...any) {
	l.write(LevelWarn, "warn", message, attrs...)
}

func (l *Logger) Error(message string, attrs ...any) {
	l.write(LevelError, "error", message, attrs...)
}

func (l *Logger) write(level Level, levelName, message string, attrs ...any) {
	if l == nil || level < l.level {
		return
	}
	event := map[string]any{
		"time":    time.Now().UTC().Format(time.RFC3339),
		"level":   levelName,
		"message": message,
	}
	for i := 0; i+1 < len(attrs); i += 2 {
		key, ok := attrs[i].(string)
		if !ok || strings.TrimSpace(key) == "" {
			continue
		}
		event[key] = attrs[i+1]
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	encoded, err := json.Marshal(event)
	if err != nil {
		fmt.Fprintf(l.out, `{"time":"%s","level":"error","message":"encode log event failed","error":%q}`+"\n", time.Now().UTC().Format(time.RFC3339), err.Error())
		return
	}
	fmt.Fprintln(l.out, string(encoded))
}
