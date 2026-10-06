package updatecheck

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"beaverdeck/internal/users"
	"beaverdeck/internal/version"
)

func TestEndpointIsOfficialBeaverDeckServer(t *testing.T) {
	if endpoint != "https://beaverdeck.io/update-check" {
		t.Fatalf("update-check endpoint = %q", endpoint)
	}
}

func TestRunOnceSendsOnlyAppVersion(t *testing.T) {
	var received map[string]any
	client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.String() != "https://test.beaverdeck.invalid/update-check" {
			t.Fatalf("unexpected URL: %s", r.URL)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"latestVersion":"1.4.2"}`)),
			Request:    r,
		}, nil
	})}

	store, err := users.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := runOnceWith(context.Background(), store, "https://test.beaverdeck.invalid/update-check", client); err != nil {
		t.Fatal(err)
	}

	if len(received) != 1 {
		t.Fatalf("expected only appVersion in update check payload, got %#v", received)
	}
	if received["appVersion"] != version.Current {
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

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
