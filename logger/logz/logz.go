// Package logz wraps go-zero logx with automatic context tag extraction.
package logz

import (
	"context"

	"github.com/parkoo/pkgo/context/tag"
	"github.com/zeromicro/go-zero/core/logx"
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

	return logx.WithContext(ctx).WithCallerSkip(1).WithFields(fields...)
}
