package control

import "path"

// Route applies the block list, then the first matching rewrite rule.
// Patterns use path.Match globs (e.g. "claude-opus-*"); malformed patterns
// are skipped (fail-open).
func (r RoutingConfig) Route(model string) (string, bool) {
	for _, pat := range r.Block {
		if ok, err := path.Match(pat, model); err == nil && ok {
			return "", true
		}
	}
	for _, rule := range r.Rules {
		if ok, err := path.Match(rule.Match, model); err == nil && ok && rule.To != "" {
			return rule.To, false
		}
	}
	return model, false
}
