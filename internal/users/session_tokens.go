package users

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

const (
	sessionTokenPrefix   = "bdkst1"
	sessionTokenTTL      = 12 * time.Hour
	sessionSweepInterval = time.Hour
)

type sessionRecord struct {
	expiresAt time.Time
	revoked   atomic.Bool
}

func (s *Store) createSession(token string, expiresAt time.Time) {
	s.sessions.Store(token, &sessionRecord{expiresAt: expiresAt})
}

func (s *Store) findSession(token string) (*sessionRecord, bool) {
	v, ok := s.sessions.Load(token)

	if !ok {
		return nil, false
	}

	return v.(*sessionRecord), true
}

func (s *Store) revokeSession(token string) {
	if rec, ok := s.findSession(token); ok {
		rec.revoked.Store(true)
	}
}

func (s *Store) Logout(token string) error {
	if _, err := s.parseSessionToken(token); err != nil {
		return nil
	}

	s.revokeSession(token)
	return nil
}

func (s *Store) sweepSessions(now time.Time) {
	s.sessions.Range(func(key, value any) bool {
		rec, ok := value.(*sessionRecord)

		if !ok || rec.revoked.Load() || !now.Before(rec.expiresAt) {
			s.sessions.Delete(key)
		}

		return true
	})
}

// Periodically clears out stale session records in the background
func (s *Store) StartSessionSweeper(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(sessionSweepInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.sweepSessions(time.Now().UTC())
			}
		}
	}()
}

type sessionTokenPayload struct {
	Username       string `json:"username"`
	AuthSource     string `json:"auth_source"`
	Role           Role   `json:"role"`
	SessionVersion int64  `json:"session_version,omitempty"`
	IssuedAt       int64  `json:"iat"`
	ExpiresAt      int64  `json:"exp"`
}

func (s *Store) newSessionToken(username, authSource string, role Role, sessionVersion int64) (token string, expiresAt time.Time, err error) {
	if len(s.sessionSigningKey) == 0 {
		return "", time.Time{}, fmt.Errorf("session signing key is not configured")
	}
	now := time.Now().UTC()
	expiresAt = now.Add(sessionTokenTTL)
	payload := sessionTokenPayload{
		Username:       strings.TrimSpace(username),
		AuthSource:     strings.TrimSpace(strings.ToLower(authSource)),
		Role:           Role(strings.TrimSpace(strings.ToLower(string(role)))),
		SessionVersion: sessionVersion,
		IssuedAt:       now.Unix(),
		ExpiresAt:      expiresAt.Unix(),
	}
	if payload.Username == "" || payload.AuthSource == "" || payload.Role == "" {
		return "", time.Time{}, fmt.Errorf("session token payload is incomplete")
	}
	payloadData, err := json.Marshal(payload)
	if err != nil {
		return "", time.Time{}, err
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadData)
	sig := s.signSessionPayload(encodedPayload)
	token = sessionTokenPrefix + "." + encodedPayload + "." + base64.RawURLEncoding.EncodeToString(sig)
	return token, expiresAt, nil
}

func (s *Store) parseSessionToken(token string) (sessionTokenPayload, error) {
	if strings.TrimSpace(token) == "" {
		return sessionTokenPayload{}, sql.ErrNoRows
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != sessionTokenPrefix {
		return sessionTokenPayload{}, sql.ErrNoRows
	}
	if len(s.sessionSigningKey) == 0 {
		return sessionTokenPayload{}, fmt.Errorf("session signing key is not configured")
	}
	expectedSig := s.signSessionPayload(parts[1])
	actualSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(actualSig, expectedSig) {
		return sessionTokenPayload{}, sql.ErrNoRows
	}
	payloadData, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return sessionTokenPayload{}, sql.ErrNoRows
	}
	var payload sessionTokenPayload
	if err := json.Unmarshal(payloadData, &payload); err != nil {
		return sessionTokenPayload{}, sql.ErrNoRows
	}
	payload.Username = strings.TrimSpace(payload.Username)
	payload.AuthSource = strings.TrimSpace(strings.ToLower(payload.AuthSource))
	payload.Role = Role(strings.TrimSpace(strings.ToLower(string(payload.Role))))
	if payload.Username == "" || payload.AuthSource == "" || payload.Role == "" {
		return sessionTokenPayload{}, sql.ErrNoRows
	}
	if payload.ExpiresAt <= time.Now().UTC().Unix() {
		return sessionTokenPayload{}, sql.ErrNoRows
	}
	return payload, nil
}

func (s *Store) signSessionPayload(encodedPayload string) []byte {
	mac := hmac.New(sha256.New, s.sessionSigningKey)
	_, _ = mac.Write([]byte(encodedPayload))
	return mac.Sum(nil)
}
