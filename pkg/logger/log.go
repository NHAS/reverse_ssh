package logger

import (
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	"strings"
)

type Urgency int

const (
	INFO Urgency = iota
	WARN
	ERROR
	FATAL
)

type Logger struct {
	id string
}

func (l *Logger) Info(format string, v ...interface{}) {
	l.Ulogf(2, INFO, format, v...)
}

func (l *Logger) Warning(format string, v ...interface{}) {
	l.Ulogf(2, WARN, format, v...)
}

func (l *Logger) Error(format string, v ...interface{}) {
	l.Ulogf(2, ERROR, format, v...)
}

func (l *Logger) Fatal(format string, v ...interface{}) {
	l.Ulogf(2, FATAL, format, v...)
}

func (l *Logger) Ulogf(callerStackDepth int, u Urgency, format string, v ...interface{}) {
	pc, file, line, ok := runtime.Caller(callerStackDepth)
	if !ok {
		file = "?"
		line = 0
	}

	fn := runtime.FuncForPC(pc)
	var fnName string
	if fn == nil {
		fnName = "?()"
	} else {
		dotName := filepath.Ext(fn.Name())
		fnName = strings.TrimLeft(dotName, ".") + "()"
	}

	msg := fmt.Sprintf(format, v...)
	prefix := fmt.Sprintf("[%s] %s %s:%d %s : ", l.id, urgency(u), filepath.Base(file), line, fnName)

	log.Print(prefix, msg, "\n")
	if u == FATAL {
		panic("Log was used with FATAL")
	}
}

func urgency(u Urgency) string {
	switch u {
	case INFO:
		return "INFO"
	case WARN:
		return "WARNING"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	}

	return "UNKNOWN_URGENCY"
}

func NewLog(id string) Logger {
	var l Logger
	l.id = id
	return l
}
