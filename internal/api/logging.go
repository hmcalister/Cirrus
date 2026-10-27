package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
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

// Logs every request that reaches the mux, and recovers a panic so that the request is still logged.
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

		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("panic while handling request",
					"request_id", requestID,
					"method", r.Method,
					"route", routePattern(r),
					"panic", recovered,
					"stack", string(debug.Stack()),
				)
				// The response may already be started, in which case the status cannot change.
				if recorder.status == 0 {
					http.Error(recorder, "internal server error", http.StatusInternalServerError)
				}
			}

			status := recorder.status
			if status == 0 {
				status = http.StatusOK
			}

			level := slog.LevelInfo
			switch {
			case status >= 500:
				level = slog.LevelError
			case status >= 400:
				level = slog.LevelWarn
			}

			slog.Log(r.Context(), level, "request",
				"request_id", requestID,
				"method", r.Method,
				"route", routePattern(r),
				"status", status,
				"bytes", recorder.bytes,
				"duration", time.Since(start),
				"remote_addr_hash", hashIdentifier(remoteHost(r.RemoteAddr)),
			)
		}()

		next.ServeHTTP(recorder, r)
	})
}

// Returns the address without its port, so that all requests from one host hash the same.
func remoteHost(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

func routePattern(r *http.Request) string {
	if r.Pattern != "" {
		return r.Pattern
	}
	return r.URL.Path
}

// Hashes an identifier so that emails and addresses do not reach the log file.
func hashIdentifier(value string) string {
	if value == "" {
		return ""
	}

	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}

func newRequestID() string {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		slog.Debug("could not read random request id", "err", err)
		return time.Now().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(raw)
}
