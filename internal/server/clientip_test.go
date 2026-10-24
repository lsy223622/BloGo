package server

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestClientIPTrustedChain(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32"), netip.MustParsePrefix("::1/128"), netip.MustParsePrefix("10.0.0.0/24")}
	for _, test := range []struct {
		remote  string
		headers []string
		want    string
	}{
		{"192.0.2.1:12", []string{"198.51.100.1"}, "192.0.2.1"},
		{"127.0.0.1:12", []string{"203.0.113.1, 198.51.100.7, 10.0.0.2"}, "198.51.100.7"},
		{"127.0.0.1:12", []string{"203.0.113.1", "198.51.100.7, 10.0.0.2"}, "198.51.100.7"},
		{"[::1]:12", []string{"2001:db8::8"}, "2001:db8::8"},
		{"[::ffff:127.0.0.1]:12", []string{"::ffff:198.51.100.7"}, "198.51.100.7"},
		{"127.0.0.1:12", []string{"198.51.100.1, invalid"}, "127.0.0.1"},
		{"127.0.0.1:12", []string{""}, "127.0.0.1"},
		{"127.0.0.1:12", nil, "127.0.0.1"},
	} {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = test.remote
		for _, header := range test.headers {
			r.Header.Add("X-Forwarded-For", header)
		}
		got, err := ClientIP(r, trusted)
		if err != nil || got.String() != test.want {
			t.Fatalf("%s %v: %s %v want %s", test.remote, test.headers, got, err, test.want)
		}
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "invalid"
	if _, err := ClientIP(r, trusted); err == nil {
		t.Fatal("invalid peer accepted")
	}
}
