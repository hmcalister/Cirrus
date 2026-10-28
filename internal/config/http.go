package config

import "time"

type HTTPConfig struct {
	Port              int
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

// Read the HTTP server configuration, which only the server uses.
func loadHTTP() (HTTPConfig, error) {
	var l loader

	cfg := HTTPConfig{
		Port:              l.int(envHTTPPort),
		ReadHeaderTimeout: l.duration(envHTTPReadHeaderTimeout),
		ReadTimeout:       l.duration(envHTTPReadTimeout),
		WriteTimeout:      l.duration(envHTTPWriteTimeout),
		IdleTimeout:       l.duration(envHTTPIdleTimeout),
		ShutdownTimeout:   l.duration(envHTTPShutdownTimeout),
	}

	return cfg, l.err()
}
