package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr                      string
	BasePath                        string
	DataDir                         string
	AppVersion                      string
	ClusterName                     string
	PodNamespace                    string
	ServiceAccountName              string
	ManagedNamespace                string
	ConfigSecretName                string
	ConfigSecretKey                 string
	ConfigSecretNS                  string
	SuppressedInsightsConfigMapName string
	SuppressedInsightsConfigMapKey  string
	SuppressedInsightsConfigMapNS   string
	AllowAllNamespaces              bool
	UpdateCheckURL                  string
	UpdateCheckEvery                time.Duration
	UpdateCheckJitter               time.Duration
	RestartDiagnosticsEnabled       bool
	RestartDiagnosticsInterval      time.Duration
	RestartDiagnosticsHistory       time.Duration
	RestartDiagnosticsMaxLogLines   int64
	RestartDiagnosticsMaxLogBytes   int64
	RestartDiagnosticsMaxTotalBytes int64
	RestartDiagnosticsMaxEvents     int
}

func FromEnv() Config {
	cfg := Config{
		ListenAddr:                      env("LISTEN_ADDR", ":8080"),
		BasePath:                        normalizeBasePath(env("BASE_PATH", "")),
		DataDir:                         env("DATA_DIR", "/data"),
		AppVersion:                      env("APP_VERSION", ""),
		ClusterName:                     env("CLUSTER_NAME", ""),
		PodNamespace:                    env("POD_NAMESPACE", "default"),
		ServiceAccountName:              env("SERVICE_ACCOUNT_NAME", "default"),
		ManagedNamespace:                env("MANAGED_NAMESPACE", env("POD_NAMESPACE", "default")),
		ConfigSecretName:                env("CONFIG_SECRET_NAME", "beaverdeck-config"),
		ConfigSecretKey:                 env("CONFIG_SECRET_KEY", "config.yaml"),
		ConfigSecretNS:                  env("CONFIG_SECRET_NAMESPACE", env("POD_NAMESPACE", "default")),
		SuppressedInsightsConfigMapName: env("SUPPRESSED_INSIGHTS_CONFIGMAP_NAME", "beaverdeck-suppressed-insights"),
		SuppressedInsightsConfigMapKey:  env("SUPPRESSED_INSIGHTS_CONFIGMAP_KEY", "suppressed_insights.json"),
		SuppressedInsightsConfigMapNS:   env("SUPPRESSED_INSIGHTS_CONFIGMAP_NAMESPACE", env("POD_NAMESPACE", "default")),
		UpdateCheckURL:                  env("UPDATE_CHECK_URL", "https://arequs.com/update-check"),
	}

	allowAll, _ := strconv.ParseBool(env("ALLOW_ALL_NAMESPACES", "false"))
	cfg.AllowAllNamespaces = allowAll

	restartDiagnosticsEnabled, err := strconv.ParseBool(env("RESTART_DIAGNOSTICS_ENABLED", "true"))
	if err != nil {
		restartDiagnosticsEnabled = true
	}
	cfg.RestartDiagnosticsEnabled = restartDiagnosticsEnabled
	cfg.RestartDiagnosticsInterval = durationFromPositiveEnv("RESTART_DIAGNOSTICS_INTERVAL_SECONDS", 10, time.Second)
	cfg.RestartDiagnosticsHistory = durationFromPositiveEnv("RESTART_DIAGNOSTICS_HISTORY_MINUTES", 5, time.Minute)
	cfg.RestartDiagnosticsMaxLogLines = int64FromPositiveEnv("RESTART_DIAGNOSTICS_MAX_LOG_LINES", 100)
	cfg.RestartDiagnosticsMaxLogBytes = int64FromPositiveEnv("RESTART_DIAGNOSTICS_MAX_LOG_BYTES_PER_CONTAINER", 32768)
	cfg.RestartDiagnosticsMaxTotalBytes = int64FromPositiveEnv("RESTART_DIAGNOSTICS_MAX_TOTAL_LOG_BYTES", 131072)
	cfg.RestartDiagnosticsMaxEvents = int(int64FromPositiveEnv("RESTART_DIAGNOSTICS_MAX_EVENTS", 20))

	intervalHours, err := strconv.Atoi(env("UPDATE_CHECK_INTERVAL_HOURS", "24"))
	if err != nil || intervalHours <= 0 {
		intervalHours = 24
	}
	cfg.UpdateCheckEvery = time.Duration(intervalHours) * time.Hour

	jitterMinutes, err := strconv.Atoi(env("UPDATE_CHECK_JITTER_MINUTES", "120"))
	if err != nil || jitterMinutes < 0 {
		jitterMinutes = 120
	}
	cfg.UpdateCheckJitter = time.Duration(jitterMinutes) * time.Minute
	return cfg
}

func durationFromPositiveEnv(key string, fallback int64, unit time.Duration) time.Duration {
	value := int64FromPositiveEnv(key, fallback)
	return time.Duration(value) * unit
}

func int64FromPositiveEnv(key string, fallback int64) int64 {
	value, err := strconv.ParseInt(env(key, strconv.FormatInt(fallback, 10)), 10, 64)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func normalizeBasePath(path string) string {
	if path == "" || path == "/" {
		return ""
	}
	return "/" + strings.Trim(path, "/")
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
