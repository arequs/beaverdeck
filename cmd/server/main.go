package main

import (
	"context"
	"embed"
	"log"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"beaverdeck/internal/api"
	"beaverdeck/internal/audit"
	"beaverdeck/internal/auth"
	"beaverdeck/internal/config"
	"beaverdeck/internal/kube"
	"beaverdeck/internal/updatecheck"
	"beaverdeck/internal/users"
)

//go:embed web/*
var webFS embed.FS

func main() {
	cfg := config.FromEnv()

	kc, err := kube.InCluster()
	if err != nil {
		log.Fatalf("kube init failed: %v", err)
	}

	auditStore, err := audit.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("audit init failed: %v", err)
	}
	defer auditStore.Close()

	userStore, err := users.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("users init failed: %v", err)
	}
	defer userStore.Close()

	bootstrapStatus, err := userStore.PrepareBootstrap(context.Background())
	if err != nil {
		log.Fatalf("users bootstrap init failed: %v", err)
	}
	if !bootstrapStatus.Initialized {
		log.Printf("beaverdeck bootstrap token: %s", bootstrapStatus.Token)
	}

	srv := api.New(cfg, kc, auditStore, userStore, webFS)

	routes := srv.Routes()
	secured := auth.Middleware(userStore)(routes)
	mux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			routes.ServeHTTP(w, r)
			return
		}
		if cfg.BasePath != "" {
			if r.Header.Get("X-Forwarded-Prefix") == "" {
				r.Header.Set("X-Forwarded-Prefix", cfg.BasePath)
			}
			if r.URL.Path == cfg.BasePath {
				http.Redirect(w, r, cfg.BasePath+"/", http.StatusMovedPermanently)
				return
			}
			if strings.HasPrefix(r.URL.Path, cfg.BasePath+"/") {
				r = r.Clone(r.Context())
				r.URL.Path = strings.TrimPrefix(r.URL.Path, cfg.BasePath)
				if r.URL.RawPath != "" {
					r.URL.RawPath = strings.TrimPrefix(r.URL.RawPath, cfg.BasePath)
				}
				if r.URL.Path == "" {
					r.URL.Path = "/"
				}
			}
		}
		if strings.HasPrefix(r.URL.Path, "/api/auth/") {
			routes.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/healthz" {
			routes.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			secured.ServeHTTP(w, r)
			return
		}
		routes.ServeHTTP(w, r)
	})
	httpServer := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	updatecheck.Start(ctx, cfg, kc, userStore)

	go func() {
		log.Printf("beaverdeck listening on %s (base_path=%s managed namespace=%s allow_all=%v)", cfg.ListenAddr, cfg.BasePath, cfg.ManagedNamespace, cfg.AllowAllNamespaces)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen failed: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
}
