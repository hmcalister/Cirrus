package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
)

const requestIDHeader = "X-Request-ID"

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (rec *statusRecorder) WriteHeader(status int) {
	if rec.status == 0 {
		rec.status = status
	}
	rec.ResponseWriter.WriteHeader(status)
}

func (rec *statusRecorder) Write(body []byte) (int, error) {
	if rec.status == 0 {
		rec.status = http.StatusOK
	}
	written, err := rec.ResponseWriter.Write(body)
	rec.bytes += written
	return written, err
}

// Logs every request that reaches the mux.
func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(requestIDHeader)
		if requestID == "" {
			requestID = newRequestID()
		}
		r = r.WithContext(withRequestID(r.Context(), requestID))
		w.Header().Set(requestIDHeader, requestID)

		start := s.now()
		recorder := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)

		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}

		slog.Info("request",
			"request_id", requestID,
			"method", r.Method,
			"route", routePattern(r),
			"status", status,
			"bytes", recorder.bytes,
			"duration", time.Since(start),
			"remote_addr", r.RemoteAddr,
		)
	})
}

func routePattern(r *http.Request) string {
	if r.Pattern != "" {
		return r.Pattern
	}
	return r.URL.Path
}

func hashEmail(userEmail string) string {
	if userEmail == "" {
		return ""
	}

	sum := sha256.Sum256([]byte(userEmail))
	return hex.EncodeToString(sum[:8])
}

func newRequestID() string {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return time.Now().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(raw)
}
