package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatusHandlerReportsDisabled(t *testing.T) {
	var cfg *Config
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/status", nil)

	cfg.StatusHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Enabled {
		t.Fatal("enabled = true, want false")
	}
}

func TestStatusHandlerReportsEnabled(t *testing.T) {
	cfg := &Config{Password: "secret"}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/status", nil)

	cfg.StatusHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Enabled {
		t.Fatal("enabled = false, want true")
	}
}
