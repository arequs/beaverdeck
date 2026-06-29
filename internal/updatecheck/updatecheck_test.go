package updatecheck

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"beaverdeck/internal/config"
	"beaverdeck/internal/users"
)

func TestRunOnceSendsOnlyAppVersion(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"latestVersion":"1.4.2"}`))
	}))
	defer server.Close()

	store, err := users.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	cfg := config.Config{
		AppVersion:       "1.4.1",
		UpdateCheckURL:   server.URL,
		UpdateCheckEvery: time.Hour,
	}
	if err := runOnce(context.Background(), cfg, store); err != nil {
		t.Fatal(err)
	}

	if len(received) != 1 {
		t.Fatalf("expected only appVersion in update check payload, got %#v", received)
	}
	if received["appVersion"] != "1.4.1" {
		t.Fatalf("unexpected appVersion payload: %#v", received)
	}
	status, err := store.GetUpdateCheckStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.LatestVersion != "1.4.2" {
		t.Fatalf("latest version not stored: %#v", status)
	}
}
