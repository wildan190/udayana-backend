package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHomePageIsServedAtRoot(t *testing.T) {
	handler := New(nil, nil).Routes()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", response.Code, http.StatusOK)
	}
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/html") {
		t.Fatalf("GET / Content-Type = %q, want text/html", contentType)
	}
	if !strings.Contains(response.Body.String(), "<title>Udayana Market API</title>") {
		t.Fatal("GET / did not serve the Udayana API landing page")
	}
	if !strings.Contains(response.Body.String(), "/api/v1/auth/register") {
		t.Fatal("GET / did not document buyer registration")
	}
}

func TestUnknownRouteRemainsNotFound(t *testing.T) {
	handler := New(nil, nil).Routes()
	request := httptest.NewRequest(http.MethodGet, "/not-an-endpoint", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("GET /not-an-endpoint status = %d, want %d", response.Code, http.StatusNotFound)
	}
}
