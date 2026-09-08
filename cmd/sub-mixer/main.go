// Command sub-mixer is a transparent reverse proxy between subscription-page and the panel
// that mixes extra connection links into subscription responses.
package main

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"

	"github.com/egor-muindor/submix/internal/config"
	"github.com/egor-muindor/submix/internal/mixer"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	panelRaw, err := config.MustString("PANEL_URL")
	if err != nil {
		return err
	}
	panelURL, err := url.Parse(panelRaw)
	if err != nil {
		return err
	}
	if panelURL.Scheme == "" || panelURL.Host == "" {
		return errors.New("PANEL_URL must be absolute, e.g. http://remnawave:3000")
	}

	extrasURL, err := config.MustString("EXTRAS_URL")
	if err != nil {
		return err
	}

	timeout, err := config.Duration("EXTRAS_TIMEOUT", 500*time.Millisecond)
	if err != nil {
		return err
	}

	interceptRaw := config.String("INTERCEPT_PATHS", `^/api/sub/([^/]+)`)
	intercept, err := regexp.Compile(interceptRaw)
	if err != nil {
		return err
	}

	listen := config.String("LISTEN", ":3020")

	proxy, err := mixer.NewProxy(mixer.ProxyConfig{
		PanelURL:       panelURL,
		Extras:         mixer.NewExtrasClient(extrasURL, timeout),
		InterceptPaths: intercept,
	})
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/", proxy)

	srv := &http.Server{
		Addr:              listen,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	slog.Info("sub-mixer started",
		"listen", listen, "panel", panelURL.String(), "extras", extrasURL,
		"intercept", interceptRaw, "extrasTimeout", timeout.String())

	return srv.ListenAndServe()
}
