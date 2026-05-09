package logging

import (
	"fmt"
	"log"
	"os"
	"strings"

	servicepkg "github.com/kardianos/service"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
)

type Level int

const (
	LevelError Level = iota
	LevelWarn
	LevelInfo
	LevelDebug
)

type Logger interface {
	Errorf(format string, args ...any)
	Warnf(format string, args ...any)
	Infof(format string, args ...any)
	Debugf(format string, args ...any)
}

type LevelLogger struct {
	level  Level
	target target
}

type target interface {
	Errorf(format string, args ...any)
	Warnf(format string, args ...any)
	Infof(format string, args ...any)
}

type stdTarget struct {
	logger *log.Logger
}

type serviceTarget struct {
	logger servicepkg.Logger
}

func NewStd(level string) Logger {
	return &LevelLogger{
		level:  parseLevel(level),
		target: stdTarget{logger: log.New(os.Stdout, "skill-organizer ", log.LstdFlags)},
	}
}

func NewForService(level string, svc servicepkg.Service) Logger {
	parsed := parseLevel(level)
	if svc != nil {
		if logger, err := svc.SystemLogger(nil); err == nil {
			return &LevelLogger{level: parsed, target: serviceTarget{logger: logger}}
		}
		if logger, err := svc.Logger(nil); err == nil {
			return &LevelLogger{level: parsed, target: serviceTarget{logger: logger}}
		}
	}
	return NewStd(level)
}

func LoadForRegistry(registryPath string, svc servicepkg.Service) Logger {
	serviceConfig, err := configpkg.LoadServiceConfigOrDefault(registryPath)
	if err != nil {
		fallback := NewForService(configpkg.DefaultLogLevel, svc)
		fallback.Warnf("failed to load service log config from %s: %v", registryPath, err)
		return fallback
	}
	return NewForService(serviceConfig.LogLevel, svc)
}

func ValidateLevel(level string) error {
	value := strings.ToLower(strings.TrimSpace(level))
	if !configpkg.IsValidLogLevel(value) {
		return fmt.Errorf("invalid log level %q; use error, warn, info, or debug", level)
	}
	return nil
}

func NormalizeLevel(level string) string {
	value := strings.ToLower(strings.TrimSpace(level))
	if !configpkg.IsValidLogLevel(value) {
		return configpkg.DefaultLogLevel
	}
	return value
}

func (l *LevelLogger) Errorf(format string, args ...any) {
	if l.level < LevelError {
		return
	}
	l.target.Errorf(format, args...)
}

func (l *LevelLogger) Warnf(format string, args ...any) {
	if l.level < LevelWarn {
		return
	}
	l.target.Warnf(format, args...)
}

func (l *LevelLogger) Infof(format string, args ...any) {
	if l.level < LevelInfo {
		return
	}
	l.target.Infof(format, args...)
}

func (l *LevelLogger) Debugf(format string, args ...any) {
	if l.level < LevelDebug {
		return
	}
	l.target.Infof("DEBUG: "+format, args...)
}

func (t stdTarget) Errorf(format string, args ...any) {
	t.logger.Printf("ERROR: "+format, args...)
}

func (t stdTarget) Warnf(format string, args ...any) {
	t.logger.Printf("WARN: "+format, args...)
}

func (t stdTarget) Infof(format string, args ...any) {
	t.logger.Printf("INFO: "+format, args...)
}

func (t serviceTarget) Errorf(format string, args ...any) {
	_ = t.logger.Errorf(format, args...)
}

func (t serviceTarget) Warnf(format string, args ...any) {
	_ = t.logger.Warningf(format, args...)
}

func (t serviceTarget) Infof(format string, args ...any) {
	_ = t.logger.Infof(format, args...)
}

func parseLevel(level string) Level {
	switch NormalizeLevel(level) {
	case "error":
		return LevelError
	case "warn":
		return LevelWarn
	case "debug":
		return LevelDebug
	default:
		return LevelInfo
	}
}
