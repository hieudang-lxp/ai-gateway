package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func serviceProxy(address string) (http.Handler, error) {
	target, err := url.Parse(address)
	if err != nil || target.Host == "" || (target.Scheme != "http" && target.Scheme != "https") {
		return nil, fmt.Errorf("invalid service URL")
	}
	forward := httputil.NewSingleHostReverseProxy(target)
	forward.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "service temporarily unavailable", http.StatusBadGateway)
	}
	return forward, nil
}
