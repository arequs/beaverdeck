package updatecheck

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"math/big"
	"net/http"
	"strings"
	"time"

	"beaverdeck/internal/config"
	"beaverdeck/internal/users"
	"beaverdeck/internal/version"
)

const endpoint = "https://beaverdeck.io/update-check"

type requestPayload struct {
	AppVersion string `json:"appVersion"`
}

type responsePayload struct {
	LatestVersion string `json:"latestVersion"`
}

func Start(ctx context.Context, cfg config.Config, userStore *users.Store) {
	go loop(ctx, cfg, userStore)
}

func loop(ctx context.Context, cfg config.Config, userStore *users.Store) {
	timer := time.NewTimer(initialDelay(cfg))
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			_ = runOnce(ctx, userStore)
			timer.Reset(nextDelay(cfg))
		}
	}
}

func runOnce(ctx context.Context, userStore *users.Store) error {
	return runOnceWith(ctx, userStore, endpoint, &http.Client{Timeout: 5 * time.Second})
}

func runOnceWith(ctx context.Context, userStore *users.Store, requestURL string, client *http.Client) error {
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	payload, err := json.Marshal(requestPayload{
		AppVersion: version.Current,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, requestURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var decoded responsePayload
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil
	}

	err = userStore.SetUpdateCheckStatus(reqCtx, users.UpdateCheckStatus{
		LatestVersion: strings.TrimSpace(decoded.LatestVersion),
	})
	if err != nil {
		return err
	}

	return nil
}

func initialDelay(cfg config.Config) time.Duration {
	jitter := cfg.UpdateCheckJitter
	if jitter <= 0 {
		return time.Minute
	}
	return randomDuration(jitter)
}

func nextDelay(cfg config.Config) time.Duration {
	base := cfg.UpdateCheckEvery
	if base <= 0 {
		base = 24 * time.Hour
	}
	jitter := cfg.UpdateCheckJitter
	if jitter <= 0 {
		return base
	}
	offset := randomDuration(2*jitter) - jitter
	delay := base + offset
	if delay < time.Hour {
		return time.Hour
	}
	return delay
}

func randomDuration(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(max.Nanoseconds()+1))
	if err != nil {
		return 0
	}
	return time.Duration(n.Int64())
}
