package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const logStreamHeartbeatInterval = 15 * time.Second

type namedLogStream struct {
	source string
	stream io.ReadCloser
	err    error
}

type logStreamEvent struct {
	source    string
	line      string
	truncated bool
	err       error
}

type logStreamPayload struct {
	Source    string `json:"source,omitempty"`
	Line      string `json:"line,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
	Error     string `json:"error,omitempty"`
}

func serveLogStreams(w http.ResponseWriter, r *http.Request, sources []namedLogStream) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, fmt.Errorf("streaming unsupported"))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, ": connected\n\n")
	flusher.Flush()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	for _, source := range sources {
		if source.stream != nil {
			defer source.stream.Close()
		}
	}

	events := make(chan logStreamEvent, max(128, len(sources)))
	var wg sync.WaitGroup
	for _, source := range sources {
		if source.err != nil {
			events <- logStreamEvent{source: source.source, err: source.err}
			continue
		}
		if source.stream == nil {
			continue
		}
		wg.Add(1)
		go func(source namedLogStream) {
			defer wg.Done()
			err := readBoundedLogLines(source.stream, maxLogStreamLineBytes, func(line string, truncated bool) error {
				select {
				case events <- logStreamEvent{source: source.source, line: line, truncated: truncated}:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			})
			if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) {
				select {
				case events <- logStreamEvent{source: source.source, err: err}:
				case <-ctx.Done():
				}
			}
		}(source)
	}
	go func() {
		wg.Wait()
		close(events)
	}()

	heartbeat := time.NewTicker(logStreamHeartbeatInterval)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if _, err := io.WriteString(w, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case event, open := <-events:
			if !open {
				_ = writeLogSSEEvent(w, "end", logStreamPayload{})
				flusher.Flush()
				return
			}
			eventName := "log"
			payload := logStreamPayload{Source: event.source, Line: event.line, Truncated: event.truncated}
			if event.err != nil {
				eventName = "error"
				payload.Error = event.err.Error()
			}
			if err := writeLogSSEEvent(w, eventName, payload); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeLogSSEEvent(w io.Writer, event string, payload logStreamPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	return err
}

func readBoundedLogLines(reader io.Reader, maxLineBytes int, emit func(line string, truncated bool) error) error {
	if maxLineBytes <= 0 {
		return fmt.Errorf("maximum log line size must be positive")
	}
	buffered := bufio.NewReaderSize(reader, 64*1024)
	line := make([]byte, 0, min(maxLineBytes, 64*1024))
	truncated := false
	for {
		fragment, continued, err := buffered.ReadLine()
		if len(fragment) > 0 && !truncated {
			remaining := maxLineBytes - len(line)
			if len(fragment) > remaining {
				line = append(line, fragment[:remaining]...)
				truncated = true
			} else {
				line = append(line, fragment...)
			}
		}
		if continued && len(line) >= maxLineBytes {
			truncated = true
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if continued {
			continue
		}
		if err := emit(string(line), truncated); err != nil {
			return err
		}
		line = line[:0]
		truncated = false
	}
}
