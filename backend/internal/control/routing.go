package control

import "path"

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
