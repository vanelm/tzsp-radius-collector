package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/vanelm/tzsp-radius-collector/internal/api"
	"github.com/vanelm/tzsp-radius-collector/internal/capture"
	"github.com/vanelm/tzsp-radius-collector/internal/catalog"
	"github.com/vanelm/tzsp-radius-collector/internal/config"
	"github.com/vanelm/tzsp-radius-collector/internal/discovery"
	"github.com/vanelm/tzsp-radius-collector/internal/forwarder"
	"github.com/vanelm/tzsp-radius-collector/internal/pipeline"
	"github.com/vanelm/tzsp-radius-collector/internal/radiusdecode"
	"github.com/vanelm/tzsp-radius-collector/internal/recorder"
	"github.com/vanelm/tzsp-radius-collector/internal/replay"
	"github.com/vanelm/tzsp-radius-collector/internal/runtime"
	"github.com/vanelm/tzsp-radius-collector/internal/store"
	"github.com/vanelm/tzsp-radius-collector/internal/stream"
	"github.com/vanelm/tzsp-radius-collector/internal/synth"
	"github.com/vanelm/tzsp-radius-collector/internal/webui"
)

func main() {
	cfg := config.Load()
	level := &slog.LevelVar{}
	level.Set(defaultLogLevel(cfg.AppEnv))
	if override, ok := parseSlogLevel(cfg.LogLevel); ok {
		level.Set(override)
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	slogger := slog.New(handler)
	slog.SetDefault(slogger)
	logger := slog.With("component", "collector")

	dict, files, err := radiusdecode.LoadAttributeDictionaryByGlob(cfg.DictionaryGlob)
	if err != nil {
		logger.Error("dictionary load failed", "error", err, "dictionary_glob", cfg.DictionaryGlob)
		os.Exit(1)
	}
	logger.Info("dictionaries loaded", "count", len(files), "dictionary_glob", cfg.DictionaryGlob)

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		logger.Error("database open failed", "error", err, "db_path", cfg.DBPath)
		os.Exit(1)
	}
	defer st.Close()

	hub := stream.NewHub(slog.With("component", "stream_hub"), cfg.SlowConsumerQueue)
	processor := pipeline.NewProcessor(dict, slog.With("component", "pipeline"))
	fwd := forwarder.New(slog.With("component", "forwarder"), st, forwarder.Config{
		AuthTarget: cfg.ForwardAuthTarget,
		AcctTarget: cfg.ForwardAcctTarget,
	})
	rec := recorder.New(slog.With("component", "recorder"), st)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	orch := runtime.NewOrchestrator(slog.With("component", "orchestrator"), hub, processor, fwd, rec, catalog.NewAutofill(slog.With("component", "catalog"), st))
	rep := replay.New(slog.With("component", "replay"), st, fwd, orch.Emit)
	syn := synth.New(slog.With("component", "synth"), st, fwd, orch.Emit)

	go hub.Run(ctx)

	if cfg.EnableMDNS {
		mdns, mdnsErr := discovery.StartMDNS(cfg, slog.With("component", "mdns"))
		if mdnsErr != nil {
			logger.Warn("mdns disabled", "error", mdnsErr)
		} else {
			defer mdns.Shutdown()
		}
	}

	events := make(chan pipeline.Event, cfg.EventQueue)
	startCaptures(ctx, cfg, slog.With("component", "capture"), events)
	go orch.Run(ctx, events)

	apiServer := api.New(slog.With("component", "api"), st, fwd, rec, rep, syn)

	mux := http.NewServeMux()
	stream.RegisterRoutes(mux, hub, cfg)
	apiServer.RegisterRoutes(mux)
	mux.Handle("/", webui.Handler())

	srv := &http.Server{
		Addr:         cfg.HTTPListen,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelShutdown()
		_ = srv.Shutdown(shutdownCtx)
	}()

	logger.Info("http listening", "listen_addr", cfg.HTTPListen, "ws_path", cfg.WSPath, "app_env", cfg.AppEnv, "db_path", cfg.DBPath)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("http server failed", "error", err)
		os.Exit(1)
	}
}

func startCaptures(ctx context.Context, cfg config.Config, logger *slog.Logger, out chan<- pipeline.Event) {
	if cfg.EnableTZSPUDP {
		listener := capture.NewTZSPUDPListener(cfg, logger.With("mode", "tzsp_udp"))
		go listener.Run(ctx, out)
	}
	if cfg.EnableRawSniff {
		sniffer := capture.NewRawSniffer(cfg, logger.With("mode", "raw_sniff"))
		go sniffer.Run(ctx, out)
	}
}

func defaultLogLevel(appEnv string) slog.Level {
	if strings.EqualFold(strings.TrimSpace(appEnv), "development") {
		return slog.LevelDebug
	}
	return slog.LevelInfo
}

func parseSlogLevel(level string) (slog.Level, bool) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug, true
	case "info":
		return slog.LevelInfo, true
	case "warn", "warning":
		return slog.LevelWarn, true
	case "error":
		return slog.LevelError, true
	default:
		return slog.LevelInfo, false
	}
}
