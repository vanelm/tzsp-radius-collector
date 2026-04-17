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

	"github.com/vanelm/tzsp-radius-collector/internal/capture"
	"github.com/vanelm/tzsp-radius-collector/internal/config"
	"github.com/vanelm/tzsp-radius-collector/internal/discovery"
	"github.com/vanelm/tzsp-radius-collector/internal/pipeline"
	"github.com/vanelm/tzsp-radius-collector/internal/radiusdecode"
	"github.com/vanelm/tzsp-radius-collector/internal/stream"
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

	hub := stream.NewHub(slog.With("component", "stream_hub"), cfg.SlowConsumerQueue)
	processor := pipeline.NewProcessor(dict, slog.With("component", "pipeline"))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

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
	go fanout(ctx, slog.With("component", "fanout"), hub, processor, events)

	mux := http.NewServeMux()
	stream.RegisterRoutes(mux, hub, cfg)

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

	logger.Info("http listening", "listen_addr", cfg.HTTPListen, "ws_path", cfg.WSPath, "app_env", cfg.AppEnv)
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

func fanout(ctx context.Context, logger *slog.Logger, hub *stream.Hub, processor *pipeline.Processor, in <-chan pipeline.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-in:
			msg, err := processor.Transform(event)
			if err != nil {
				logger.Debug("event transform skipped", "error", err, "source", event.Source)
				continue
			}
			hub.Publish(msg)
		}
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
