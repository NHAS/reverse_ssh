//go:build nologging

package logger

func (l *Logger) Ulogf(callerStackDepth int, u Urgency, format string, v ...interface{}) {

}
