// Command extras-api is the source of extra connection links for sub-mixer.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/egor-muindor/submix/internal/config"
	"github.com/egor-muindor/submix/internal/extras"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := config.String("CONFIG", "/etc/submix/extras.yaml")
	cfg, err := extras.Load(configPath)
	if err != nil {
		return err
	}

	cacheDir := config.String("CACHE_DIR", "/var/lib/submix")
	store, err := extras.NewStore(cacheDir)
	if err != nil {
		return err
	}

	names := make([]string, 0, len(cfg.Subscriptions))
	for _, sub := range cfg.Subscriptions {
		names = append(names, sub.Name)
	}
	if err := store.Restore(names); err != nil {
		return err
	}

	panelURL, err := config.MustString("PANEL_URL")
	if err != nil {
		return err
	}
	panelToken, err := config.MustString("PANEL_TOKEN")
	if err != nil {
		return err
	}
	panelTTL, err := config.Duration("PANEL_CACHE_TTL", 5*time.Minute)
	if err != nil {
		return err
	}
	panelTimeout, err := config.Duration("PANEL_TIMEOUT", 3*time.Second)
	if err != nil {
		return err
	}
	fetchTimeout, err := config.Duration("FETCH_TIMEOUT", 30*time.Second)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fetcher := extras.NewFetcher(store, fetchTimeout)
	for _, sub := range cfg.Subscriptions {
		go fetcher.Run(ctx, sub)
	}

	panel := extras.NewPanelClient(panelURL, panelToken, panelTTL, panelTimeout)
	server := extras.NewServer(cfg, store, panel)

	listen := config.String("LISTEN", ":3030")
	srv := &http.Server{
		Addr:              listen,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.Info("extras-api started",
		"listen", listen, "config", configPath, "cacheDir", cacheDir,
		"subscriptions", len(cfg.Subscriptions), "staticEntries", len(cfg.StaticEntries),
		"panelCacheTTL", panelTTL.String())

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
