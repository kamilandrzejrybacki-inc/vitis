package api

import (
	"net/http"
	"strings"
)

// corsMiddleware adds CORS headers. If origin is empty, defaults to "*".
func corsMiddleware(origin string, next http.Handler) http.Handler {
	if origin == "" {
		origin = "*"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// apiKeyMiddleware enforces Bearer token auth for non-localhost requests.
// If key is empty, all requests are allowed.
func apiKeyMiddleware(key string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if key == "" {
			next.ServeHTTP(w, r)
			return
		}
		host := r.RemoteAddr
		if idx := strings.LastIndex(host, ":"); idx != -1 {
			host = host[:idx]
		}
		// Allow localhost without auth
		if host == "127.0.0.1" || host == "::1" || host == "localhost" {
			next.ServeHTTP(w, r)
			return
		}
		// Require Bearer token
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != key {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
