package main

import (
	"context"
	"embed"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"beaverdeck/internal/api"
	"beaverdeck/internal/auth"
	"beaverdeck/internal/config"
	"beaverdeck/internal/kube"
	"beaverdeck/internal/updatecheck"
	"beaverdeck/internal/users"
)

//go:embed web/*
var webFS embed.FS

func main() {
	if handled, err := handleUtilityCommand(os.Args[1:], os.Stdin, os.Stdout); handled {
		if err != nil {
			log.Fatalf("%v", err)
		}
		return
	}

	cfg := config.FromEnv()

	kc, err := kube.InCluster()
	if err != nil {
		log.Fatalf("kube init failed: %v", err)
	}

	userStore, err := users.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("users init failed: %v", err)
	}
	defer userStore.Close()

	configSecretRef := kube.ConfigSecretRef{
		Namespace: cfg.ConfigSecretNS,
		Name:      cfg.ConfigSecretName,
		Key:       cfg.ConfigSecretKey,
	}
	bootstrapStatus, err := initializeUserConfig(context.Background(), kc, userStore, configSecretRef)
	if err != nil {
		log.Fatalf("users bootstrap init failed: %v", err)
	}
	if !bootstrapStatus.Initialized {
		log.Printf("beaverdeck initialization required: open the UI and set the initial admin username and password")
	}

	srv := api.New(cfg, kc, userStore, webFS)

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
	updatecheck.Start(ctx, cfg, userStore)

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

func initializeUserConfig(ctx context.Context, kc *kube.Client, userStore *users.Store, ref kube.ConfigSecretRef) (users.BootstrapStatus, error) {
	log.Printf("beaverdeck auth config secret: %s", ref.String())
	data, found, err := kc.GetConfigSecretData(ctx, ref)
	if err != nil {
		return users.BootstrapStatus{}, err
	}

	imported := false
	if found && strings.TrimSpace(string(data)) != "" {
		log.Printf("beaverdeck auth config secret found; importing existing configuration from %s", ref.String())
		snapshot, err := users.DecodeConfigSnapshotBytes(data)
		if err != nil {
			stage := firstNonEmpty(users.ConfigImportStage(err), "decode")
			log.Printf("beaverdeck auth config import failed at %s: %v", stage, err)
			logConfigSecretRecoveryHint(ref)
			return users.BootstrapStatus{}, fmt.Errorf("auth config secret import failed at %s: %w", stage, err)
		}
		normalized, err := users.NormalizeConfigSnapshot(snapshot)
		if err != nil {
			stage := firstNonEmpty(users.ConfigImportStage(err), "normalize")
			log.Printf("beaverdeck auth config import failed at %s: %v", stage, err)
			logConfigSecretRecoveryHint(ref)
			return users.BootstrapStatus{}, fmt.Errorf("auth config secret import failed at %s: %w", stage, err)
		} else if err := userStore.ImportConfigSnapshot(ctx, normalized); err != nil {
			stage := firstNonEmpty(users.ConfigImportStage(err), "apply")
			log.Printf("beaverdeck auth config import failed at %s: %v", stage, err)
			logConfigSecretRecoveryHint(ref)
			return users.BootstrapStatus{}, fmt.Errorf("auth config secret import failed at %s: %w", stage, err)
		} else {
			imported = true
			log.Printf(
				"beaverdeck auth config import succeeded: initialized=%v users=%d roles=%d google_mappings=%d oidc_mappings=%d",
				normalized.Initialized,
				len(normalized.Users),
				len(normalized.Roles),
				len(normalized.Google.Mappings),
				len(normalized.OIDC.Mappings),
			)
		}
	} else if found {
		err := fmt.Errorf("auth config secret %s exists but key %q is empty", ref.String(), ref.Key)
		log.Printf("beaverdeck auth config import failed at decode: %v", err)
		logConfigSecretRecoveryHint(ref)
		return users.BootstrapStatus{}, err
	} else {
		log.Printf("beaverdeck auth config secret not found; starting empty runtime configuration")
	}

	if !imported {
		if err := userStore.ResetToEmptyConfig(ctx); err != nil {
			return users.BootstrapStatus{}, err
		}
	}

	userStore.SetConfigSaver(func(ctx context.Context, snapshot users.ConfigSnapshot) error {
		data, err := users.EncodeConfigSnapshot(snapshot)
		if err != nil {
			return err
		}
		return kc.UpsertConfigSecretData(ctx, ref, data)
	})

	bootstrapStatus, err := userStore.PrepareBootstrap(ctx)
	if err != nil {
		return users.BootstrapStatus{}, err
	}
	if imported {
		if err := userStore.PersistConfig(ctx); err != nil {
			return users.BootstrapStatus{}, err
		}
		log.Printf("beaverdeck auth config secret normalized after successful import")
	} else {
		log.Printf("beaverdeck auth config bootstrap is pending; Secret will be created after successful initialization")
	}
	return bootstrapStatus, nil
}

func logConfigSecretRecoveryHint(ref kube.ConfigSecretRef) {
	log.Printf("beaverdeck auth config recovery: fix Secret %s, or delete Secret %s/%s to start bootstrap initialization again", ref.String(), ref.Namespace, ref.Name)
}

func handleUtilityCommand(args []string, stdin io.Reader, stdout io.Writer) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}
	switch args[0] {
	case "hash-password":
		password := ""
		if len(args) > 1 {
			password = args[1]
		} else {
			data, err := io.ReadAll(io.LimitReader(stdin, 4096))
			if err != nil {
				return true, fmt.Errorf("read password from stdin: %w", err)
			}
			password = strings.TrimRight(string(data), "\r\n")
		}
		hash, err := users.HashLocalPassword(password)
		if err != nil {
			return true, err
		}
		_, err = fmt.Fprintln(stdout, hash)
		return true, err
	default:
		return false, nil
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
