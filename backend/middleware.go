package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
)

type Middleware func(http.Handler) http.Handler

func ChainMiddleware(h http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

func FormatIPPort(addr string) string {
	// addr is usually in form "IP:port"
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		// fallback, just return addr as is
		return addr
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return addr
	}

	// IPv4 max length is 15
	ipStr := ip.String()
	ipFixed := fmt.Sprintf("%-15s", ipStr)

	// port with colon, pad to 6 chars ":80   "
	portFixed := fmt.Sprintf(":%-5s", port)

	return ipFixed + portFixed
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixedAddr := FormatIPPort(r.RemoteAddr)
		log.Printf("(%s) [%s] %s", fixedAddr, r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
