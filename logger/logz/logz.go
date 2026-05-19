// Package logz wraps go-zero logx with automatic context tag extraction.
package logz

import (
	"context"
	"time"

	"github.com/parkoo/pkgo/context/tag"

	"github.com/zeromicro/go-zero/core/logx"
	gormlogger "gorm.io/gorm/logger"
)

// WithCtx returns a logx.Logger enriched with tag fields from ctx.
// Note: the caller's ctx is not updated; tags should be set before calling this.
func WithCtx(ctx context.Context) logx.Logger {
	ctx, tags := tag.Extract(ctx)

	values := tags.Values()
	fields := make([]logx.LogField, 0, len(values))
	for k, v := range values {
		fields = append(fields, logx.LogField{
			Key:   k,
			Value: v,
		})
	}

	return logx.WithContext(ctx).WithCallerSkip(0).WithFields(fields...)
}

// gormLogger adapts logz to gorm's logger interface.
type gormLogger struct {
	level         gormlogger.LogLevel
	slowThreshold time.Duration
}

// NewGormLogger creates a gorm logger backed by logz.
func NewGormLogger(level gormlogger.LogLevel, slowThreshold time.Duration) gormlogger.Interface {
	return &gormLogger{
		level:         level,
		slowThreshold: slowThreshold,
	}
}

func (g *gormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	return &gormLogger{level: level, slowThreshold: g.slowThreshold}
}

func (g *gormLogger) Info(ctx context.Context, msg string, args ...interface{}) {
	if g.level >= gormlogger.Info {
		log := WithCtx(ctx)
		log.Infof("[GORM Info] "+msg, args...)
	}
}

func (g *gormLogger) Warn(ctx context.Context, msg string, args ...interface{}) {
	if g.level >= gormlogger.Warn {
		log := WithCtx(ctx)
		log.Slowf("[GORM Warn] "+msg, args...)
	}
}

func (g *gormLogger) Error(ctx context.Context, msg string, args ...interface{}) {
	if g.level >= gormlogger.Error {
		log := WithCtx(ctx)
		log.Errorf("[GORM Error] "+msg, args...)
	}
}

func (g *gormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if g.level <= gormlogger.Silent {
		return
	}

	log := WithCtx(ctx)
	elapsed := time.Since(begin)
	sql, rows := fc()

	elapsed_ms := float64(elapsed.Nanoseconds()) / 1e6

	switch {
	case err != nil && g.level >= gormlogger.Error:
		log.Errorf("[GORM Trace] "+"sql error, err: %v, elapsed: %.3fms, rows: %d, sql: %s", err, elapsed_ms, rows, sql)
	case elapsed > g.slowThreshold && g.slowThreshold > 0 && g.level >= gormlogger.Warn:
		log.Slowf("[GORM Trace] "+"slow sql, elapsed: %.3fms, threshold: %v, rows: %d, sql: %s", elapsed_ms, g.slowThreshold, rows, sql)
	case g.level >= gormlogger.Info:
		log.Infof("[GORM Trace] "+"sql trace, elapsed: %.3fms, rows: %d, sql: %s", elapsed_ms, rows, sql)
	}
}
