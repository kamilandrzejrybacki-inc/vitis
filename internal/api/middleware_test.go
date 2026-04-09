package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func TestCORSMiddleware_AddsHeaders(t *testing.T) {
	handler := corsMiddleware("https://example.com", http.HandlerFunc(okHandler))
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, "https://example.com", rr.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, OPTIONS", rr.Header().Get("Access-Control-Allow-Methods"))
}

func TestCORSMiddleware_DefaultsToStar(t *testing.T) {
	handler := corsMiddleware("", http.HandlerFunc(okHandler))
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSMiddleware_HandlesPreflightOPTIONS(t *testing.T) {
	handler := corsMiddleware("*", http.HandlerFunc(okHandler))
	req := httptest.NewRequest("OPTIONS", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestAPIKeyMiddleware_RequiresBearerKey(t *testing.T) {
	handler := apiKeyMiddleware("secret-key", http.HandlerFunc(okHandler))
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAPIKeyMiddleware_AllowsValidBearerKey(t *testing.T) {
	handler := apiKeyMiddleware("secret-key", http.HandlerFunc(okHandler))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer secret-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAPIKeyMiddleware_AllowsAllWhenNoKeyConfigured(t *testing.T) {
	handler := apiKeyMiddleware("", http.HandlerFunc(okHandler))
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.5:12345"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}
