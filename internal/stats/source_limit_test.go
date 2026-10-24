package stats

import (
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/lsy223622/BloGo/internal/requestmeta"
)

func TestCollectorLimitsByResolvedIP(t *testing.T) {
	h, _, _, _ := apiFixture(t)
	send := func(ip string, trusted bool) int {
		r := httptest.NewRequest("POST", "/api/events", strings.NewReader(eventBody("a", "post:a", "direct")))
		r.Header.Set("Content-Type", "application/json")
		r.RemoteAddr = "192.0.2.1:10"
		r.Header.Set("X-Forwarded-For", ip)
		if trusted {
			r = requestmeta.WithClientIP(r, netip.MustParseAddr(ip))
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	for i := 0; i < 40; i++ {
		if got := send("198.51.100.1", true); got != 204 {
			t.Fatal(got)
		}
	}
	if got := send("198.51.100.1", true); got != 429 {
		t.Fatal("source limiter missing", got)
	}
	if got := send("198.51.100.2", true); got != 204 {
		t.Fatal("different real source shares bucket", got)
	}
	for i := 0; i < 40; i++ {
		if got := send("203.0.113.1", false); got != 204 {
			t.Fatal(got)
		}
	}
	if got := send("203.0.113.2", false); got != 429 {
		t.Fatal("untrusted XFF changed bucket", got)
	}
	now := time.Now()
	limits := sourceLimits{}
	for i := 0; i < 4096; i++ {
		if !limits.allow(time.Unix(int64(i), 0).String(), now) {
			t.Fatal("unexpected full limiter")
		}
	}
	if limits.allow("extra", now) {
		t.Fatal("unbounded source state")
	}
	if !limits.allow("fresh", now.Add(11*time.Minute)) || len(limits.buckets) != 1 {
		t.Fatal("idle sources not collected")
	}
}
