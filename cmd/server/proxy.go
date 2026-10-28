package main

import (
	"net/http"
	"net/netip"

	"github.com/hmcalister/stratus/pkg/auth"
)

var trustedProxies = []netip.Prefix{
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"),
}

func getClientIPMapperFunc(debug bool) func(*http.Request) string {
	if !debug {
		return auth.TrustedProxyClientIP(trustedProxies)
	}

	return nil
}
