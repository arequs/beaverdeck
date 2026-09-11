package users

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestSessionRecordCreateFindRevoke(t *testing.T) {
	store := &Store{}
	const token = "test-token"

	if _, ok := store.findSession(token); ok {
		t.Fatal("expected no session record before creation")
	}

	store.createSession(token, time.Now().Add(time.Hour))
	rec, ok := store.findSession(token)

	if !ok {
		t.Fatal("expected to find session record after creation")
	}

	if rec.revoked.Load() {
		t.Fatal("newly created session must not start revoked")
	}

	store.revokeSession(token)
	rec, ok = store.findSession(token)

	if !ok || !rec.revoked.Load() {
		t.Fatal("expected session record to be revoked")
	}
}

func TestCreateSessionRegistersRecordForLocalAndExternalSources(t *testing.T) {
	ctx := context.Background()
	store, err := Open(t.TempDir())

	if err != nil {
		t.Fatal(err)
	}

	defer store.Close()

	if err := store.CompleteBootstrap(ctx, "admin", "admin-pass"); err != nil {
		t.Fatal(err)
	}

	localToken, err := store.CreateSession(ctx, "admin", "local")

	if err != nil {
		t.Fatal(err)
	}

	if _, ok := store.findSession(localToken); !ok {
		t.Fatal("expected local login to register a session record findable by its token")
	}

	externalToken, err := store.CreateExternalSession(ctx, "user@example.com", "google", RoleAdmin)

	if err != nil {
		t.Fatal(err)
	}

	if _, ok := store.findSession(externalToken); !ok {
		t.Fatal("expected external login to register a session record findable by its token")
	}
}

func TestAuthenticateRejectsRevokedSessionDespiteValidSignatureAndExpiry(t *testing.T) {
	ctx := context.Background()
	store, err := Open(t.TempDir())

	if err != nil {
		t.Fatal(err)
	}

	defer store.Close()

	if err := store.CompleteBootstrap(ctx, "admin", "admin-pass"); err != nil {
		t.Fatal(err)
	}

	token, err := store.CreateSession(ctx, "admin", "local")

	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.Authenticate(ctx, token); err != nil {
		t.Fatalf("expected freshly issued token to authenticate, got %v", err)
	}

	store.revokeSession(token)

	if _, err := store.Authenticate(ctx, token); err != sql.ErrNoRows {
		t.Fatalf("expected revoked session to be rejected with sql.ErrNoRows, got %v", err)
	}
}

func TestSweepSessionsRemovesRevokedAndExpiredButKeepsLive(t *testing.T) {
	store := &Store{}
	now := time.Now()

	store.createSession("revoked", now.Add(time.Hour))
	store.revokeSession("revoked")

	store.createSession("expired", now.Add(-time.Minute))

	store.createSession("live", now.Add(time.Hour))

	store.sweepSessions(now)

	if _, ok := store.findSession("revoked"); ok {
		t.Fatal("expected revoked session to be swept")
	}

	if _, ok := store.findSession("expired"); ok {
		t.Fatal("expected expired session to be swept")
	}

	if _, ok := store.findSession("live"); !ok {
		t.Fatal("expected live session to survive the sweep")
	}
}
