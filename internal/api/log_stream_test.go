package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadBoundedLogLinesTruncatesAndContinues(t *testing.T) {
	input := "first\n\n" + strings.Repeat("x", 40) + "\nafter"
	type capturedLine struct {
		text      string
		truncated bool
	}
	var lines []capturedLine
	err := readBoundedLogLines(strings.NewReader(input), 16, func(line string, truncated bool) error {
		lines = append(lines, capturedLine{text: line, truncated: truncated})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []capturedLine{
		{text: "first"},
		{text: ""},
		{text: strings.Repeat("x", 16), truncated: true},
		{text: "after"},
	}
	if len(lines) != len(want) {
		t.Fatalf("captured %d lines, want %d: %#v", len(lines), len(want), lines)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("line %d = %#v, want %#v", i, lines[i], want[i])
		}
	}
}

func TestWriteLogSSEEventUsesJSONPayload(t *testing.T) {
	var output bytes.Buffer
	payload := logStreamPayload{Source: "pod-a", Line: "line with\ncontrol", Truncated: true}
	if err := writeLogSSEEvent(&output, "log", payload); err != nil {
		t.Fatal(err)
	}
	raw := output.String()
	if !strings.HasPrefix(raw, "event: log\ndata: ") || !strings.HasSuffix(raw, "\n\n") {
		t.Fatalf("unexpected SSE frame: %q", raw)
	}
	encoded := strings.TrimSuffix(strings.TrimPrefix(raw, "event: log\ndata: "), "\n\n")
	var decoded logStreamPayload
	if err := json.Unmarshal([]byte(encoded), &decoded); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if decoded != payload {
		t.Fatalf("decoded payload = %#v, want %#v", decoded, payload)
	}
}

func TestRequestedFollowLogTailAllowsZero(t *testing.T) {
	tests := []struct {
		query string
		want  int64
	}{
		{query: "?follow=true&tail=0", want: 0},
		{query: "?follow=true&tail=400", want: 400},
		{query: "?follow=true&tail=999999", want: maxLogTailLines},
		{query: "?follow=true", want: defaultLogTailLines},
	}
	for _, tt := range tests {
		request := httptest.NewRequest(http.MethodGet, "/api/podlogs"+tt.query, nil)
		if got := requestedFollowLogTail(request); got != tt.want {
			t.Fatalf("requestedFollowLogTail(%q) = %d, want %d", tt.query, got, tt.want)
		}
	}
}

func TestServeLogStreamsWritesStreamingHeadersAndEvents(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/podlogs?follow=true", nil)
	recorder := httptest.NewRecorder()
	serveLogStreams(recorder, request, []namedLogStream{{
		source: "pod-a",
		stream: io.NopCloser(strings.NewReader("first\nsecond\n")),
	}})

	response := recorder.Result()
	defer response.Body.Close()
	if got := response.Header.Get("Content-Type"); got != "text/event-stream; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := response.Header.Get("Cache-Control"); got != "no-cache, no-transform" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := response.Header.Get("X-Accel-Buffering"); got != "no" {
		t.Fatalf("X-Accel-Buffering = %q", got)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, expected := range []string{
		": connected\n\n",
		"event: log\ndata: {\"source\":\"pod-a\",\"line\":\"first\"}\n\n",
		"event: log\ndata: {\"source\":\"pod-a\",\"line\":\"second\"}\n\n",
		"event: end\ndata: {}\n\n",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("stream output does not contain %q: %q", expected, text)
		}
	}
}
