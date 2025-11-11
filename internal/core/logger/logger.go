package logger

import (
	"fmt"
	"log/slog"
)

type logger_level struct {
	lvl slog.Level
}

type Logger interface {
	Printf(msg string, args ...any)

	Infof(msg string, args ...any)
	Info(msg string, args ...any)
	Debugf(msg string, args ...any)
	Debug(msg string, args ...any)
	Errorf(msg string, args ...any)
	Error(msg string, args ...any)
}

type loggerImpl struct {
	s *slog.Logger
}

func (l *loggerImpl) Printf(msg string, args ...any) {
	fmt.Println(fmt.Sprintf(msg, args...))
}
func (l *loggerImpl) Infof(msg string, args ...any) {
	l.s.Info(fmt.Sprintf(msg, args...))
}
func (l *loggerImpl) Info(msg string, args ...any) {
	l.s.Info(msg, args...)
}
func (l *loggerImpl) Debugf(msg string, args ...any) {
	l.s.Debug(fmt.Sprintf(msg, args...))
}
func (l *loggerImpl) Debug(msg string, args ...any) {
	l.s.Debug(msg, args...)
}
func (l *loggerImpl) Errorf(msg string, args ...any) {
	l.s.Error(fmt.Sprintf(msg, args...))
}
func (l *loggerImpl) Error(msg string, args ...any) {
	l.s.Error(msg, args...)
}

func init() {
	slog.SetLogLoggerLevel(globalLevel.Level())
}

func NewLogger(cmd ...string) Logger {

	slogger := slog.Default()
	if len(cmd) > 0 {
		slogger = slogger.WithGroup(cmd[0])
	}

	return &loggerImpl{slogger}
}
