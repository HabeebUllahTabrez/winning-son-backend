package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ZapRequestLogger is a middleware that logs requests using zap.
func ZapRequestLogger(logger *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				// Check if the logger is in development mode
				isDev := logger.Core().Enabled(zapcore.DebugLevel)

				fields := []zap.Field{
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
					zap.Int("status", ww.Status()),
					zap.Int("bytes", ww.BytesWritten()),
					zap.Duration("duration", time.Since(start)),
					zap.String("remote_ip", r.RemoteAddr),
				}

				if reqID := middleware.GetReqID(r.Context()); reqID != "" {
					fields = append(fields, zap.String("request_id", reqID))
				}

				if isDev {
					// In development, log in a more readable format
					msg := fmt.Sprintf("%s %s %d %s",
						r.Method, r.URL.Path, ww.Status(), time.Since(start))
					logger.Info(msg, fields...)
				} else {
					// In production, log as structured JSON
					logger.Info("request completed", fields...)
				}
			}()

			next.ServeHTTP(ww, r)
		})
	}
}
