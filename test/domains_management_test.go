package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/api/handlers/management"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func newDomainsTestHandler(t *testing.T, domains []string) (*management.Handler, string) {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := &config.Config{
		Domains: config.NormalizeDomains(domains),
	}

	if err := os.WriteFile(configPath, []byte("port: 8080\n"), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	h := management.NewHandler(cfg, configPath, nil)
	return h, configPath
}

func setupDomainsRouter(h *management.Handler) *gin.Engine {
	r := gin.New()
	mgmt := r.Group("/v0/management")
	{
		mgmt.GET("/domains", h.GetDomains)
		mgmt.PUT("/domains", h.PutDomains)
		mgmt.PATCH("/domains", h.PatchDomains)
		mgmt.DELETE("/domains", h.DeleteDomains)
	}
	return r
}

func TestGetDomainsReturnsEmptyArray(t *testing.T) {
	h, _ := newDomainsTestHandler(t, nil)
	r := setupDomainsRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v0/management/domains", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp struct {
		Domains []string `json:"domains"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(resp.Domains) != 0 {
		t.Fatalf("expected empty domains list, got %#v", resp.Domains)
	}
}

func TestPutPatchDeleteDomainsPersistsNormalizedValues(t *testing.T) {
	h, configPath := newDomainsTestHandler(t, nil)
	r := setupDomainsRouter(h)

	putBody := `[" *.ICOA.qzz.io ", "", "*.icoe.pp.ua", "*.icoa.qzz.io"]`
	req := httptest.NewRequest(http.MethodPut, "/v0/management/domains", bytes.NewBufferString(putBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	loaded, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config from disk: %v", err)
	}
	want := []string{"*.icoa.qzz.io", "*.icoe.pp.ua"}
	if len(loaded.Domains) != len(want) {
		t.Fatalf("expected %d domains after put, got %d", len(want), len(loaded.Domains))
	}
	for i := range want {
		if loaded.Domains[i] != want[i] {
			t.Fatalf("expected domain %d to be %q, got %q", i, want[i], loaded.Domains[i])
		}
	}

	patchBody := `{"old":"*.icoe.pp.ua","new":" *.icoa.pp.ua "}`
	req = httptest.NewRequest(http.MethodPatch, "/v0/management/domains", bytes.NewBufferString(patchBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d from patch, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	loaded, err = config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to reload config from disk: %v", err)
	}
	want = []string{"*.icoa.qzz.io", "*.icoa.pp.ua"}
	if len(loaded.Domains) != len(want) {
		t.Fatalf("expected %d domains after patch, got %d", len(want), len(loaded.Domains))
	}
	for i := range want {
		if loaded.Domains[i] != want[i] {
			t.Fatalf("expected domain %d to be %q, got %q", i, want[i], loaded.Domains[i])
		}
	}

	req = httptest.NewRequest(http.MethodDelete, "/v0/management/domains?value=*.icoa.qzz.io", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d from delete, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	loaded, err = config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to reload config after delete: %v", err)
	}
	want = []string{"*.icoa.pp.ua"}
	if len(loaded.Domains) != len(want) {
		t.Fatalf("expected %d domains after delete, got %d", len(want), len(loaded.Domains))
	}
	if loaded.Domains[0] != want[0] {
		t.Fatalf("expected remaining domain %q, got %q", want[0], loaded.Domains[0])
	}
}
