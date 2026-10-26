package server

import (
	"bytes"
	"net/http/httptest"
	"testing"
)

func TestFeedEntrypointsAreDirectAndTyped(t *testing.T) {
	h, _, _, _ := httpFixture(t)
	var rss []byte
	for _, test := range []struct{ path, kind string }{{"/rss.xml", "application/rss+xml; charset=utf-8"}, {"/feed/", "application/rss+xml; charset=utf-8"}, {"/feed/rss/", "application/rdf+xml; charset=utf-8"}, {"/feed/atom/", "application/atom+xml; charset=utf-8"}} {
		for _, method := range []string{"GET", "HEAD"} {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(method, test.path, nil))
			if w.Code != 200 || w.Header().Get("Content-Type") != test.kind || w.Header().Get("Location") != "" {
				t.Fatalf("%s %s: %d %v", method, test.path, w.Code, w.Header())
			}
			if method == "HEAD" && w.Body.Len() != 0 {
				t.Fatal("HEAD body")
			}
			if method == "GET" {
				if test.path == "/rss.xml" {
					rss = append([]byte(nil), w.Body.Bytes()...)
				}
				if test.path == "/feed/" && !bytes.Equal(rss, w.Body.Bytes()) {
					t.Fatal("RSS2 alias differs")
				}
			}
		}
	}
	for _, path := range []string{"/feed/comment/", "/feed/tags/tech/"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Fatal(path, w.Code)
		}
	}
}
