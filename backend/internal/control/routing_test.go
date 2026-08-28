package control

import "testing"

func TestRoute(t *testing.T) {
	r := RoutingConfig{
		Rules: []Rule{
			{Match: "claude-opus-4-8", To: "claude-haiku-4-5-20251001"}, // exact, first
			{Match: "claude-opus-*", To: "claude-sonnet-5"},
		},
		Block: []string{"claude-fable-*"},
	}
	cases := []struct {
		in, want string
		blocked  bool
	}{
		{"claude-fable-5", "", true},                            // block wins
		{"claude-opus-4-8", "claude-haiku-4-5-20251001", false}, // first match wins
		{"claude-opus-4-7", "claude-sonnet-5", false},           // glob
		{"claude-sonnet-5", "claude-sonnet-5", false},           // passthrough
	}
	for _, c := range cases {
		to, blocked := r.Route(c.in)
		if blocked != c.blocked || (!blocked && to != c.want) {
			t.Fatalf("Route(%q) = (%q,%v) want (%q,%v)", c.in, to, blocked, c.want, c.blocked)
		}
	}
}

func TestRouteBadPatternIgnored(t *testing.T) {
	r := RoutingConfig{Rules: []Rule{{Match: "[", To: "x"}}, Block: []string{"["}}
	to, blocked := r.Route("claude-sonnet-5")
	if blocked || to != "claude-sonnet-5" {
		t.Fatalf("bad pattern must be ignored, got (%q,%v)", to, blocked)
	}
}

func TestRouteEmptyConfigPassthrough(t *testing.T) {
	to, blocked := RoutingConfig{}.Route("m")
	if blocked || to != "m" {
		t.Fatal("empty config must pass through")
	}
}
