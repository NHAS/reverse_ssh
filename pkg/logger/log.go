package logger

import (
	"fmt"
	"log"
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

func (l *Logger) Logf(format string, v ...interface{}) {
	l.Ulogf(INFO, format, v...)
}

func (l *Logger) Ulogf(u Urgency, format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	prefix := fmt.Sprintf("[%s] [%s] ", l.id, urgency(u))

	log.Print(prefix, msg)
}

func urgency(u Urgency) string {
	switch u {
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
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
