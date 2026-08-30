package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/websocket"
)

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

type websocketWriter struct {
	conn *websocket.Conn
}

func (w *websocketWriter) Write(p []byte) (int, error) {
	if err := w.conn.WriteMessage(websocket.TextMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func writeJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]any{"error": errString(err)})
}
