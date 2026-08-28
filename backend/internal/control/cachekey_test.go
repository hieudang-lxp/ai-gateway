package control

import "testing"

func TestCacheKey(t *testing.T) {
	a := CacheKey([]byte(`{"model":"m","messages":[]}`))
	b := CacheKey([]byte(`{"model":"m","messages":[]}`))
	c := CacheKey([]byte(`{"model":"m2","messages":[]}`))
	if a != b {
		t.Fatal("same body must hash equal")
	}
	if a == c {
		t.Fatal("different body must hash different")
	}
	if len(a) != 64 {
		t.Fatalf("want sha256 hex (64 chars), got %d", len(a))
	}
}
