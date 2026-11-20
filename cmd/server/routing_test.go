package main

import (
	"net/http/httptest"
	"testing"
)

func TestConfiguredBasePathOverridesForwardedPrefix(t *testing.T) {
	request := httptest.NewRequest("GET", "http://internal/beaverdeck/", nil)
	request.Header.Set("X-Forwarded-Prefix", `\evil.example`)
	applyConfiguredBasePath(request, "/beaverdeck")
	if got, want := request.Header.Get("X-Forwarded-Prefix"), "/beaverdeck"; got != want {
		t.Fatalf("got prefix %q, want %q", got, want)
	}
}

func TestEmptyBasePathLeavesForwardedPrefixForValidation(t *testing.T) {
	request := httptest.NewRequest("GET", "http://internal/", nil)
	request.Header.Set("X-Forwarded-Prefix", "/proxy")
	applyConfiguredBasePath(request, "")
	if got, want := request.Header.Get("X-Forwarded-Prefix"), "/proxy"; got != want {
		t.Fatalf("got prefix %q, want %q", got, want)
	}
}
