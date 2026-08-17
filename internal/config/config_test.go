package config

import (
	"testing"
	"time"
)

func TestRestartDiagnosticsDefaultsAndOverrides(t *testing.T) {
	t.Setenv("RESTART_DIAGNOSTICS_ENABLED", "")
	t.Setenv("RESTART_DIAGNOSTICS_INTERVAL_SECONDS", "")
	t.Setenv("RESTART_DIAGNOSTICS_HISTORY_MINUTES", "")
	t.Setenv("RESTART_DIAGNOSTICS_MAX_LOG_LINES", "")
	t.Setenv("RESTART_DIAGNOSTICS_MAX_LOG_BYTES_PER_CONTAINER", "")
	t.Setenv("RESTART_DIAGNOSTICS_MAX_TOTAL_LOG_BYTES", "")
	t.Setenv("RESTART_DIAGNOSTICS_MAX_EVENTS", "")

	cfg := FromEnv()
	if !cfg.RestartDiagnosticsEnabled || cfg.RestartDiagnosticsInterval != 10*time.Second || cfg.RestartDiagnosticsHistory != 5*time.Minute {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.RestartDiagnosticsMaxLogLines != 100 || cfg.RestartDiagnosticsMaxLogBytes != 32768 || cfg.RestartDiagnosticsMaxTotalBytes != 131072 || cfg.RestartDiagnosticsMaxEvents != 20 {
		t.Fatalf("unexpected safety caps: %+v", cfg)
	}

	t.Setenv("RESTART_DIAGNOSTICS_ENABLED", "false")
	t.Setenv("RESTART_DIAGNOSTICS_INTERVAL_SECONDS", "15")
	t.Setenv("RESTART_DIAGNOSTICS_HISTORY_MINUTES", "8")
	t.Setenv("RESTART_DIAGNOSTICS_MAX_LOG_LINES", "55")
	cfg = FromEnv()
	if cfg.RestartDiagnosticsEnabled || cfg.RestartDiagnosticsInterval != 15*time.Second || cfg.RestartDiagnosticsHistory != 8*time.Minute || cfg.RestartDiagnosticsMaxLogLines != 55 {
		t.Fatalf("unexpected overrides: %+v", cfg)
	}
}
