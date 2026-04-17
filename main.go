package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/elin/tzsp-radius-collector/internal/capture"
	"github.com/elin/tzsp-radius-collector/internal/config"
	"github.com/elin/tzsp-radius-collector/internal/discovery"
	"github.com/elin/tzsp-radius-collector/internal/pipeline"
	"github.com/elin/tzsp-radius-collector/internal/radiusdecode"
	"github.com/elin/tzsp-radius-collector/internal/stream"
)

func main() {
	cfg := config.Load()
	logger := log.New(os.Stdout, "collector: ", log.LstdFlags|log.Lmicroseconds)

	dict, files, err := radiusdecode.LoadAttributeDictionaryByGlob(cfg.DictionaryGlob)
	if err != nil {
		logger.Fatalf("dictionary load failed: %v", err)
	}
	logger.Printf("loaded dictionaries: %d", len(files))

	hub := stream.NewHub(logger, cfg.SlowConsumerQueue)
	processor := pipeline.NewProcessor(dict, logger)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go hub.Run(ctx)

	if cfg.EnableMDNS {
		mdns, mdnsErr := discovery.StartMDNS(cfg, logger)
		if mdnsErr != nil {
			logger.Printf("mdns disabled (startup error): %v", mdnsErr)
		} else {
			defer mdns.Shutdown()
		}
	}

	events := make(chan pipeline.Event, cfg.EventQueue)
	startCaptures(ctx, cfg, logger, events)
	go fanout(ctx, hub, processor, events)

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

	logger.Printf("http listening on %s", cfg.HTTPListen)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Fatalf("http server failed: %v", err)
	}
}

func startCaptures(ctx context.Context, cfg config.Config, logger *log.Logger, out chan<- pipeline.Event) {
	if cfg.EnableTZSPUDP {
		listener := capture.NewTZSPUDPListener(cfg, logger)
		go listener.Run(ctx, out)
	}
	if cfg.EnableRawSniff {
		sniffer := capture.NewRawSniffer(cfg, logger)
		go sniffer.Run(ctx, out)
	}
}

func fanout(ctx context.Context, hub *stream.Hub, processor *pipeline.Processor, in <-chan pipeline.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-in:
			msg, err := processor.Transform(event)
			if err != nil {
				continue
			}
			hub.Publish(msg)
		}
	}
}
