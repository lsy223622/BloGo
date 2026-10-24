package server

import (
	"fmt"
	"net/http"
	"net/netip"
	"strings"

	"github.com/lsy223622/BloGo/internal/requestmeta"
)

func ClientIP(r *http.Request, trusted []netip.Prefix) (netip.Addr, error) {
	peer := requestmeta.ClientIP(r)
	if !peer.IsValid() {
		return netip.Addr{}, fmt.Errorf("invalid connection peer")
	}
	isTrusted := func(ip netip.Addr) bool {
		for _, prefix := range trusted {
			if prefix.Contains(ip) {
				return true
			}
		}
		return false
	}
	if !isTrusted(peer) {
		return peer, nil
	}
	values := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	if len(values) > 64 {
		return peer, nil
	}
	current := peer
	for i := len(values) - 1; i >= 0; i-- {
		if !isTrusted(current) {
			return current, nil
		}
		next, err := netip.ParseAddr(strings.TrimSpace(values[i]))
		if err != nil || next.Zone() != "" {
			return peer, nil
		}
		current = next.Unmap()
	}
	return current, nil
}

func TrustedProxies(values []string) ([]netip.Prefix, error) {
	result := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			address, e := netip.ParseAddr(value)
			if e != nil || address.Zone() != "" {
				return nil, fmt.Errorf("invalid proxy %q", value)
			}
			address = address.Unmap()
			prefix = netip.PrefixFrom(address, address.BitLen())
		}
		if prefix.Addr().Is4In6() && prefix.Bits() >= 96 {
			prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
		}
		result = append(result, prefix.Masked())
	}
	return result, nil
}
