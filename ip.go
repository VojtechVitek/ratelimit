package ratelimiter

import (
	"net"
	"net/http"
	"strings"
)

func IP(r *http.Request) string {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexAny(xff, ",;"); i != -1 {
			xff = xff[:i]
		}
		ip += "," + xff
	}
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		ip += "," + xrip
	}
	return ip
}
