package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr         string
	BasePath           string
	AdminToken         string
	DataDir            string
	AppVersion         string
	ClusterName        string
	PodNamespace       string
	ServiceAccountName string
	ManagedNamespace   string
	AllowAllNamespaces bool
	UpdateCheckURL     string
	UpdateCheckEvery   time.Duration
	UpdateCheckJitter  time.Duration
}

func FromEnv() Config {
	cfg := Config{
		ListenAddr:         env("LISTEN_ADDR", ":8080"),
		BasePath:           normalizeBasePath(env("BASE_PATH", "")),
		AdminToken:         env("ADMIN_TOKEN", ""),
		DataDir:            env("DATA_DIR", "/data"),
		AppVersion:         env("APP_VERSION", ""),
		ClusterName:        env("CLUSTER_NAME", ""),
		PodNamespace:       env("POD_NAMESPACE", "default"),
		ServiceAccountName: env("SERVICE_ACCOUNT_NAME", "default"),
		ManagedNamespace:   env("MANAGED_NAMESPACE", env("POD_NAMESPACE", "default")),
		UpdateCheckURL:     env("UPDATE_CHECK_URL", "https://arequs.com/update-check"),
	}

	allowAll, _ := strconv.ParseBool(env("ALLOW_ALL_NAMESPACES", "false"))
	cfg.AllowAllNamespaces = allowAll

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
