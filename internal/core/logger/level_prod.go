//go:build !dev

package logger

import "log/slog"

var globalLevel slog.Leveler = slog.LevelWarn
