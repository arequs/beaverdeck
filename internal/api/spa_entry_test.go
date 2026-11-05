package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beaverdeck/internal/config"
)

func TestSPAEntryRuntimeBase(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(`<html><head><script src="./assets/app.js"></script></head></html>`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, configured, forwarded, want string }{
		{"root", "", "", `/`},
		{"configured", "/beaverdeck", "/ignored", `/beaverdeck/`},
		{"forwarded", "", "/proxy", `/proxy/`},
		{"escaped", "", `/x"<y`, `/x&#34;&lt;y/`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{cfg: config.Config{BasePath: tc.configured}, fs: http.Dir(dir)}
			r := httptest.NewRequest(http.MethodGet, "/events", nil)
			r.Header.Set("X-Forwarded-Prefix", tc.forwarded)
			w := httptest.NewRecorder()
			s.serveSPA(w, r)
			if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `<base href="`+tc.want+`">`) {
				t.Fatalf("unexpected entry: status %d, body %s", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), `name="beaverdeck-base-path"`) || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("missing runtime prefix or cache protection")
			}
		})
	}
}

func TestSPAFallbackDistinguishesNavigationAndAssets(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(`<html><head></head><body>app</body></html>`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "logo.png"), []byte("asset"), 0600); err != nil {
		t.Fatal(err)
	}
	s := &Server{fs: http.Dir(dir)}
	for _, tc := range []struct {
		path, accept string
		status       int
		body         string
	}{
		{"/", "text/html", 200, "app"},
		{"/nested/page.with.dot/", "text/html", 200, "app"},
		{"/nested/page.with.dot", "text/html", 200, "app"},
		{"/index.html", "text/html", 200, "beaverdeck-base-path"},
		{"/logo.png", "image/png", 200, "asset"},
		{"/assets/missing.js", "text/html", 404, "404"},
		{"/missing.png", "image/png", 404, "404"},
		{"/api/missing", "text/html", 404, "404"},
		{"/healthz/missing", "text/html", 404, "404"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tc.path, nil)
			r.Header.Set("Accept", tc.accept)
			w := httptest.NewRecorder()
			s.Routes().ServeHTTP(w, r)
			if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.body) {
				t.Fatalf("status %d, body %q", w.Code, w.Body.String())
			}
		})
	}
}
