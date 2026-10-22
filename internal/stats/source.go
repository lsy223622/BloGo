package stats

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/idna"
)

func normalizeHost(raw string) (string, error) {
	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
		raw = raw[1 : len(raw)-1]
	}
	if ip := net.ParseIP(raw); ip != nil {
		return ip.String(), nil
	}
	if raw == "" || strings.ContainsAny(raw, "/:@?#%\\[] \t\r\n\x00") {
		return "", fmt.Errorf("expected host without URL, credentials or port")
	}
	host, err := idna.Lookup.ToASCII(strings.TrimSuffix(raw, "."))
	if err != nil {
		return "", err
	}
	host = strings.ToLower(host)
	if len(host) == 0 || len(host) > 253 {
		return "", fmt.Errorf("invalid hostname length")
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", fmt.Errorf("invalid hostname label")
		}
		for _, ch := range label {
			if !(ch >= 'a' && ch <= 'z') && !(ch >= '0' && ch <= '9') && ch != '-' {
				return "", fmt.Errorf("invalid hostname character")
			}
		}
	}
	return host, nil
}

func NormalizeSource(raw, siteHost string) (string, error) {
	if raw == "" {
		return "direct", nil
	}
	host, err := normalizeHost(raw)
	if err != nil {
		return "", err
	}
	internal, err := normalizeHost(siteHost)
	if err != nil {
		return "", err
	}
	if host == internal {
		return "internal", nil
	}
	return host, nil
}

func normalizedOrigin(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || strings.ContainsAny(raw, "?#") || (u.Path != "" && u.Path != "/") {
		return "", fmt.Errorf("expected HTTP(S) origin")
	}
	host, err := normalizeHost(u.Hostname())
	if err != nil {
		return "", err
	}
	port := u.Port()
	if port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "", fmt.Errorf("invalid origin port")
		}
		if (u.Scheme == "https" && n == 443) || (u.Scheme == "http" && n == 80) {
			port = ""
		}
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port != "" {
		host += ":" + port
	}
	return u.Scheme + "://" + host, nil
}
