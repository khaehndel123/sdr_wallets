package log

import (
	"context"

	"go.uber.org/zap"
)

type ctxMarkerLogger struct{}

var (
	ctxKeyLogger = &ctxMarkerLogger{}
	nullLogger   = zap.NewNop().Sugar()
)

type ctxLogger struct {
	logger *zap.SugaredLogger
	fields []interface{}
}

// AddFields adds zap fields to the logger.
func AddFields(ctx context.Context, fields ...interface{}) {
	l, ok := ctx.Value(ctxKeyLogger).(*ctxLogger)
	if !ok || l == nil {
		return
	}
	l.fields = append(l.fields, fields...)
}

// ExtractLogger takes the call-scoped Logger from zap middleware.
func ExtractLogger(ctx context.Context) *zap.SugaredLogger {
	l, ok := ctx.Value(ctxKeyLogger).(*ctxLogger)
	if !ok || l == nil {
		return nullLogger
	}
	// add zap fields added until now
	return l.logger.With(l.fields...)
}

// ToContext adds the zap.Logger to the context for extraction later.
// Returning the new context that has been created.
func ToContext(ctx context.Context, logger *zap.SugaredLogger) context.Context {
	l := &ctxLogger{logger: logger}
	return context.WithValue(ctx, ctxKeyLogger, l)
}
