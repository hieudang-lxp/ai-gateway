package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServiceProxyPreservesSearchAndDetailRequests(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(r.URL.RequestURI())) }))
	defer up.Close()
	h, err := serviceProxy(up.URL)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/_sessions?q=repo&source=codex", "/_sessions/detail?source=codex&session_id=s1", "/_insights?period=month"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		body, _ := io.ReadAll(w.Result().Body)
		if w.Code != 200 || string(body) != path {
			t.Fatalf("proxy changed request: %d %s", w.Code, body)
		}
	}
}
