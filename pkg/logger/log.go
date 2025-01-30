package logger

import (
	"fmt"
	"strings"
)

type Urgency int

const (
	DISABLE         = 0
	INFO    Urgency = iota
	WARN
	ERROR
	FATAL
)

var globalLevel Urgency = INFO

type Logger struct {
	id string
}

func SetLogLevel(level Urgency) {
	globalLevel = level
}

func GetLogLevel() Urgency {
	return globalLevel
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
	case DISABLE:
		return "DISABLED"
	}

	return "UNKNOWN_URGENCY"
}

func StrToUrgency(s string) (Urgency, error) {
	s = strings.ToUpper(s)

	switch s {
	case "INFO":
		return INFO, nil
	case "WARNING", "WARN":
		return WARN, nil
	case "ERROR", "ERR":
		return ERROR, nil
	case "FATAL":
		return FATAL, nil
	case "DISABLED":
		return DISABLE, nil
	}

	return 0, fmt.Errorf("urgency %q isnt a valid urgency [INFO,WARNING,ERROR,FATAL,DISABLED]", s)
}

func UrgencyToStr(u Urgency) string {
	return urgency(u)
}

func NewLog(id string) Logger {
	var l Logger
	l.id = id
	return l
}
