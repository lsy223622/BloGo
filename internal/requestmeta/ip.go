package requestmeta

import (
	"context"
	"net/http"
	"net/netip"
)

type clientIPKey struct{}

func WithClientIP(r *http.Request, ip netip.Addr) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), clientIPKey{}, ip.Unmap()))
}

func ClientIP(r *http.Request) netip.Addr {
	if ip, ok := r.Context().Value(clientIPKey{}).(netip.Addr); ok {
		return ip
	}
	if peer, err := netip.ParseAddrPort(r.RemoteAddr); err == nil {
		return peer.Addr().Unmap()
	}
	peer, _ := netip.ParseAddr(r.RemoteAddr)
	return peer.Unmap()
}
