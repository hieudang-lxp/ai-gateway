package store

import (
	"bytes"
	"testing"
	"time"
)

func TestCachePutGet(t *testing.T) {
	st := open(t)
	c := CachedResponse{Status: 200, ContentType: "application/json",
		Body: []byte(`{"ok":true}`), CostUSD: 0.0123, Model: "claude-sonnet-5"}
	if err := st.CachePut("k1", c); err != nil {
		t.Fatal(err)
	}
	got, ok, err := st.CacheGet("k1", time.Hour)
	if err != nil || !ok {
		t.Fatalf("miss: ok=%v err=%v", ok, err)
	}
	if got.Status != 200 || got.ContentType != c.ContentType ||
		!bytes.Equal(got.Body, c.Body) || got.CostUSD != c.CostUSD || got.Model != c.Model {
		t.Fatalf("roundtrip: %+v", got)
	}
}

func TestCacheMissAndExpiry(t *testing.T) {
	st := open(t)
	if _, ok, _ := st.CacheGet("absent", time.Hour); ok {
		t.Fatal("want miss")
	}
	if err := st.CachePut("k", CachedResponse{Status: 200, Body: []byte("x")}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, ok, _ := st.CacheGet("k", time.Nanosecond); ok {
		t.Fatal("want expired entry to miss")
	}
}

func TestCachePutOverwrites(t *testing.T) {
	st := open(t)
	_ = st.CachePut("k", CachedResponse{Status: 200, Body: []byte("v1")})
	if err := st.CachePut("k", CachedResponse{Status: 200, Body: []byte("v2")}); err != nil {
		t.Fatal(err)
	}
	got, ok, _ := st.CacheGet("k", time.Hour)
	if !ok || string(got.Body) != "v2" {
		t.Fatal("want upsert to v2")
	}
}
