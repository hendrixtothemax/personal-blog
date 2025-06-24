package main

import (
    "log"
    "net"
    "net/http"
    "fmt"
)

func formatIPPort(addr string) string {
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
        fixedAddr := formatIPPort(r.RemoteAddr)
        log.Printf("(%s) [%s] %s", fixedAddr, r.Method, r.URL.Path)
        next.ServeHTTP(w, r)
    })
}
