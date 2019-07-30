package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/middleware"

	"backend/pkg/log"
)

func ZapLogger(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor) // save a response status
		logCtx := log.ToContext(r.Context(), log.Default())     // put a logger copy into a request context

		next.ServeHTTP(ww, r.WithContext(logCtx))

		logger := log.ExtractLogger(logCtx) // update the logger for the current request
		logger.Infow(
			r.Host+r.RequestURI,
			"status", ww.Status(),
			"ip", r.RemoteAddr,
			"latency", time.Since(start),
		)
	}
	return http.HandlerFunc(fn)
}
