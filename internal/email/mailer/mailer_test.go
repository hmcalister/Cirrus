package mailer

import "testing"

func TestNewServerAuthRejectsMissingFields(t *testing.T) {
	type cfgData struct {
		host     string
		port     string
		username string
		password string
	}
	tests := []struct {
		name string
		cfg  cfgData
	}{
		{
			"missing host",
			cfgData{
				host:     "",
				port:     "465",
				username: "username",
				password: "secret",
			},
		},
		{
			"missing port",
			cfgData{
				host:     "hostname",
				port:     "",
				username: "username",
				password: "secret",
			},
		},
		{
			"missing username",
			cfgData{
				host:     "hostname",
				port:     "465",
				username: "",
				password: "secret",
			},
		},
		{
			"missing password",
			cfgData{
				host:     "hostname",
				port:     "465",
				username: "username",
				password: "",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewServerAuth(tc.cfg.host, tc.cfg.port, tc.cfg.username, tc.cfg.password)
			if err == nil {
				t.Errorf("expected an error for missing required config %v", tc.name)
			}
		})
	}
}
